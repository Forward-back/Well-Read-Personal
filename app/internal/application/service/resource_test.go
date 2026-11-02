package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newResourceCatalogForTest(t *testing.T) (interfaces.ResourceCatalog, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.StoredResource{}, &types.ResourceBinding{}))
	return NewResourceCatalog(repository.NewResourceRepository(db)), db
}

func TestResourceCatalogRegisterResolveAndDeduplicate(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	physical := "storage://backend-a/local://7/exports/a.png"

	ref, err := catalog.Register(ctx, 7, physical, interfaces.ResourceRegistration{Kind: "image", OriginalName: "a.png"})
	require.NoError(t, err)
	require.Regexp(t, `^resource://[0-9A-Za-z_-]{22}$`, ref)

	again, err := catalog.Register(ctx, 7, physical, interfaces.ResourceRegistration{})
	require.NoError(t, err)
	require.Equal(t, ref, again)

	resolvedPath, resource, err := catalog.ResolvePath(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, physical, resolvedPath)
	require.Equal(t, uint64(7), resource.TenantID)
	require.Equal(t, "backend-a", resource.StorageBackendID)
	require.Equal(t, "local", resource.Provider)
}

func TestResourceCatalogBindingAndAccessGrant(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(
		ctx,
		9,
		"local://9/exports/report.pdf",
		interfaces.ResourceRegistration{OriginalName: "report.pdf"},
	)
	require.NoError(t, err)
	require.NoError(t, catalog.Bind(ctx, ref, "knowledge", "knowledge-1", "source_file"))

	token, err := catalog.CreateAccessGrant(ctx, ref, time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(token, "."), "stateless token is payload.signature")

	resource, err := catalog.ResolveAccessGrant(ctx, token)
	require.NoError(t, err)
	require.Equal(t, uint64(9), resource.TenantID)
}

// Two resources must never share a grant.
func TestResourceCatalogGrantsArePerResource(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	first, err := catalog.Register(ctx, 9, "local://9/exports/a.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)
	second, err := catalog.Register(ctx, 9, "local://9/exports/b.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	firstToken, err := catalog.CreateAccessGrant(ctx, first, time.Hour)
	require.NoError(t, err)
	secondToken, err := catalog.CreateAccessGrant(ctx, second, time.Hour)
	require.NoError(t, err)
	require.NotEqual(t, firstToken, secondToken)

	firstResource, err := catalog.ResolveAccessGrant(ctx, firstToken)
	require.NoError(t, err)
	require.Equal(t, first, types.BuildResourcePath(firstResource.Handle))
}

func TestResourceCatalogRejectsTamperedGrant(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(ctx, 9, "local://9/exports/a.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	token, err := catalog.CreateAccessGrant(ctx, ref, time.Hour)
	require.NoError(t, err)
	tampered := token[:len(token)-1] + "x"

	_, err = catalog.ResolveAccessGrant(ctx, tampered)
	require.Error(t, err)
}

func TestResourceCatalogRejectsExpiredGrant(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(ctx, 9, "local://9/exports/a.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	token, err := catalog.CreateAccessGrant(ctx, ref, time.Nanosecond)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	_, err = catalog.ResolveAccessGrant(ctx, token)
	require.Error(t, err)
}

func TestResourceCatalogRejectsGrantForDeletedResource(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(ctx, 9, "local://9/exports/a.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	token, err := catalog.CreateAccessGrant(ctx, ref, time.Hour)
	require.NoError(t, err)
	require.NoError(t, catalog.MarkDeleted(ctx, ref))

	_, err = catalog.ResolveAccessGrant(ctx, token)
	require.Error(t, err)
}

// The two-owner case that "save this answer to the knowledge base" creates:
// one blob, claimed by both the assistant message and the new document.
// Deleting either owner must leave the other's copy intact.
func TestResourceCatalogReleaseKeepsFileWhileAnotherOwnerClaimsIt(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(ctx, 7, "local://7/exports/chart.html", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	require.NoError(t, catalog.Bind(ctx, ref, types.ResourceOwnerMessage, "msg-1", types.ResourceRelationArtifact))
	require.NoError(t, catalog.Bind(ctx, ref, types.ResourceOwnerKnowledge, "kn-1", types.ResourceRelationAttachment))

	remaining, err := catalog.Release(ctx, ref, types.ResourceOwnerKnowledge, "kn-1")
	require.NoError(t, err)
	require.EqualValues(t, 1, remaining, "the message still shows this file")

	remaining, err = catalog.Release(ctx, ref, types.ResourceOwnerMessage, "msg-1")
	require.NoError(t, err)
	require.EqualValues(t, 0, remaining, "the last claim is gone, the bytes can go too")
}

// Binding twice is how a republished document re-takes its claim, and it must
// not inflate the count into a file that can never be collected.
func TestResourceCatalogBindIsIdempotentPerOwner(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()
	ref, err := catalog.Register(ctx, 7, "local://7/exports/a.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)

	require.NoError(t, catalog.Bind(ctx, ref, types.ResourceOwnerKnowledge, "kn-1", types.ResourceRelationAttachment))
	require.NoError(t, catalog.Bind(ctx, ref, types.ResourceOwnerKnowledge, "kn-1", types.ResourceRelationAttachment))

	remaining, err := catalog.Release(ctx, ref, types.ResourceOwnerKnowledge, "kn-1")
	require.NoError(t, err)
	require.EqualValues(t, 0, remaining)
}

// Releasing something that was never bound, or is not a handle at all, must be
// harmless: callers release optimistically from a content scan.
func TestResourceCatalogReleaseUnknownReference(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	ctx := context.Background()

	remaining, err := catalog.Release(ctx, "local://7/exports/legacy.png", types.ResourceOwnerKnowledge, "kn-1")
	require.NoError(t, err)
	require.EqualValues(t, -1, remaining, "a raw provider path has no claims to account for")

	ref, err := catalog.Register(ctx, 7, "local://7/exports/b.png", interfaces.ResourceRegistration{})
	require.NoError(t, err)
	remaining, err = catalog.Release(ctx, ref, types.ResourceOwnerKnowledge, "never-bound")
	require.NoError(t, err)
	require.EqualValues(t, 0, remaining)

	_, err = catalog.Release(ctx, ref, "", "kn-1")
	require.Error(t, err, "an owner is required to release a claim")
}

func TestResourceCatalogRejectsUnsupportedPhysicalPath(t *testing.T) {
	catalog, _ := newResourceCatalogForTest(t)
	_, err := catalog.Register(context.Background(), 7, "https://example.com/a.png", interfaces.ResourceRegistration{})
	require.ErrorContains(t, err, "unsupported provider")
}
