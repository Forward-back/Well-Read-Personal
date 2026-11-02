package service

import (
	"context"
	"testing"

	"gaze/internal/application/access"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeMessageFileRequiresTenantMatchAndPersistedReference(t *testing.T) {
	catalog, db := newResourceCatalogForTest(t)
	require.NoError(t, db.AutoMigrate(&types.KnowledgeBase{}, &types.Knowledge{}))
	ctx := types.WithCaller(
		context.Background(),
		types.Caller{TenantID: 7, UserID: "owner", Role: types.TenantRoleOwner},
	)
	const physical = "local://7/exports/report.pdf"
	ref, err := catalog.Register(ctx, 7, physical, interfaces.ResourceRegistration{})
	require.NoError(t, err)
	message := &types.Message{
		ID:            "message",
		AgentTenantID: 7,
		Content:       "see " + ref,
		Artifacts:     types.MessageArtifacts{{URL: ref}},
	}

	file, err := access.AuthorizeMessageFile(ctx, message, ref, catalog)
	require.NoError(t, err)
	require.Equal(t, uint64(7), file.OwnerTenantID)

	_, err = access.AuthorizeMessageFile(ctx, message, "resource://OtherReferenceValueXyz", catalog)
	require.ErrorIs(t, err, access.ErrForbidden)

	foreign := types.WithCaller(
		context.Background(),
		types.Caller{TenantID: 8, UserID: "intruder", Role: types.TenantRoleOwner},
	)
	_, err = access.AuthorizeMessageFile(foreign, message, ref, catalog)
	require.ErrorIs(t, err, access.ErrForbidden)

	_, err = access.AuthorizeMessageFile(context.Background(), message, ref, catalog)
	require.ErrorIs(t, err, access.ErrUnauthorized)
}
