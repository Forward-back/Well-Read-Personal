package service

import (
	"context"
	"testing"

	"gaze/internal/application/repository"
	apperrors "gaze/internal/errors"
	"gaze/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSessionScopeContext(tenantID uint64, userID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	if userID != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	}
	return ctx
}

func testAPISessionScopeContext(tenantID uint64, externalUserID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-7")
	return types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPIExternalUser,
		ID:   externalUserID,
	})
}

func testAPITenantKeyScopeContext(tenantID uint64, keyID uint64) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
	ctx = types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPITenant,
		ID:   "1",
	})
	return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KeyID: keyID})
}

func newTestSessionService(t *testing.T) (*sessionService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// A :memory: database lives on one connection; pinning the pool keeps the
	// migrated schema visible to every later query.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&types.Session{}))

	return &sessionService{
		sessionRepo: repository.NewSessionRepository(db),
	}, db
}

func TestGetSessionIsScopedToCurrentUser(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   "alice",
		Title:    "alice private session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(bobSession).Error)
	legacySession := &types.Session{
		TenantID: 1,
		Title:    "legacy tenant session",
	}
	require.NoError(t, db.Create(legacySession).Error)

	_, err := svc.GetSession(testSessionScopeContext(1, "bob"), aliceSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := svc.GetSession(testSessionScopeContext(1, "bob"), bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)

	got, err = svc.GetSession(testSessionScopeContext(1, "bob"), legacySession.ID)
	require.NoError(t, err)
	require.Equal(t, legacySession.ID, got.ID)
}

func TestUpdateSessionIsScopedToCurrentUserAndAllowsNoOp(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "alice private session",
		Description: "original description",
	}
	require.NoError(t, db.Create(aliceSession).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "bob"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       "bob update attempt",
		Description: "should not be saved",
	})
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	var unchanged types.Session
	require.NoError(t, db.First(&unchanged, "id = ?", aliceSession.ID).Error)
	require.Equal(t, aliceSession.Title, unchanged.Title)
	require.Equal(t, aliceSession.Description, unchanged.Description)

	err = svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       aliceSession.Title,
		Description: aliceSession.Description,
	})
	require.NoError(t, err)
}

func TestUpdateSessionRejectsPlantedMaintenanceDescription(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "alice private session",
		Description: "ordinary",
	}
	require.NoError(t, db.Create(aliceSession).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       "still mine",
		Description: types.SkillMaintenanceSessionMarker + "install",
	})
	require.NoError(t, err)

	var got types.Session
	require.NoError(t, db.First(&got, "id = ?", aliceSession.ID).Error)
	require.Equal(t, "still mine", got.Title)
	require.Empty(t, got.Description,
		"a client must not be able to hide a session behind the maintenance marker")
}

func TestUpdateSessionPreservesMaintenanceDescription(t *testing.T) {
	svc, db := newTestSessionService(t)
	marker := types.SkillMaintenanceSessionMarker + "install"
	row := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "Skill install",
		Description: marker,
	}
	require.NoError(t, db.Create(row).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          row.ID,
		TenantID:    1,
		Title:       "unhide me",
		Description: "plain chat",
	})
	require.NoError(t, err)

	var got types.Session
	require.NoError(t, db.First(&got, "id = ?", row.ID).Error)
	require.Equal(t, marker, got.Description,
		"a PUT must not strip the marker off a real maintenance session")
}

func TestGetSessionAPIExternalUserScopeAndChannelFallback(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "7:alice",
		Title:    "alice api session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "7:bob",
		Title:    "bob api session",
	}
	require.NoError(t, db.Create(bobSession).Error)
	tenantSession := &types.Session{
		TenantID: 1,
		UserID:   "system-7",
		Title:    "tenant api session",
	}
	require.NoError(t, db.Create(tenantSession).Error)

	// Any API-created session is readable in the single-owner edition, even
	// when another external user created it.
	got, err := svc.GetSession(testAPISessionScopeContext(1, "7:alice"), bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)

	got, err = svc.GetSession(testAPISessionScopeContext(1, "7:alice"), aliceSession.ID)
	require.NoError(t, err)
	require.Equal(t, aliceSession.ID, got.ID)

	// Legacy tenant sessions are not channel-managed, so the runtime scope
	// still hides them from an external-user principal.
	_, err = svc.GetSession(testAPISessionScopeContext(1, "7:alice"), tenantSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err = svc.GetSession(testSessionScopeContext(1, "system-7"), tenantSession.ID)
	require.NoError(t, err)
	require.Equal(t, tenantSession.ID, got.ID)
}

func TestGetSessionAllowsOwnerToOpenAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)
	otherUserSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(otherUserSession).Error)

	// The single owner may open API-key sessions without an admin role.
	ownerCtx := testSessionScopeContext(1, "alice")
	got, err := svc.GetSession(ownerCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// The same fallback is limited to API-created sessions; another user's
	// personal session stays hidden.
	_, err = svc.GetSession(ownerCtx, otherUserSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetOwnedSessionDeniesAdminOnAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	adminCtx := context.WithValue(
		testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleOwner,
	)

	// The read path lets an admin open the API-key session (folder navigation)...
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// ...but the strict owner scope used by write/mutation endpoints (title
	// generation, attachments, stop, QA) denies it, so admins stay read-only.
	_, err = svc.GetOwnedSession(adminCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestListSessionsAPISourceReturnsAllChannelKeys(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	key1 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:10", Title: "key1"}
	key2 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:20", Title: "key2"}
	externalUser := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	web := &types.Session{TenantID: 1, UserID: "alice", Title: "alice web"}
	require.NoError(t, db.Create(key1).Error)
	require.NoError(t, db.Create(key2).Error)
	require.NoError(t, db.Create(externalUser).Error)
	require.NoError(t, db.Create(web).Error)

	// The personal edition has one owner: the API source filter exposes every
	// API-key session in the tenant without an admin role.
	ownerCtx := testSessionScopeContext(1, "alice")
	result, err := svc.ListSessions(ownerCtx, &types.SessionListQuery{Source: types.SessionSourceAPI})
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Total)
}

func TestGetSessionAllowsOwnerToReadAPIExternalUserSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	require.NoError(t, db.Create(apiSession).Error)

	ownerCtx := testSessionScopeContext(1, "alice")
	got, err := svc.GetSession(ownerCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}

func TestListSessionsIMSourceReturnsAllChannelSessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	// IM source listing is tenant-wide for the single owner.
	ownerCtx := testSessionScopeContext(1, "alice")
	result, err := svc.ListSessions(ownerCtx, &types.SessionListQuery{Source: "feishu"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
}

func TestGetSessionAllowsOwnerOnIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	ownerCtx := testSessionScopeContext(1, "alice")
	got, err := svc.GetSession(ownerCtx, imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

func TestGetSessionAllowsAPITenantRuntimeToReadOwnAPIKeySession(t *testing.T) {
	svc, db := newTestSessionService(t)

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	got, err := svc.GetSession(testAPITenantKeyScopeContext(1, 10), apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}

func TestGetSessionAllowsAPITenantRuntimeToReadIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	// The personal edition keeps channel sessions readable tenant-wide; the
	// API runtime is not denied the IM row.
	got, err := svc.GetSession(testAPITenantKeyScopeContext(1, 10), imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

func TestGetSessionAllowsAPIExternalUserRuntimeToReadIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	got, err := svc.GetSession(testAPISessionScopeContext(1, "1:alice"), imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

func TestGetSessionAllowsIMRuntimeToReadIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID,
		Platform:  "feishu",
	}).Error)

	ctx := context.WithValue(
		testSessionScopeContext(1, "system-1"),
		types.TenantRoleContextKey,
		types.TenantRoleOwner,
	)
	ctx = types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalIMUser,
		ID:   "1:channel-1:feishu:open-id-1",
	})

	got, err := svc.GetSession(ctx, imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

// testListSessionsIMChannelSession lets QueryPaged's LEFT JOIN resolve against a
// real table in the in-memory SQLite database.
type testListSessionsIMChannelSession struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	SessionID   string `gorm:"column:session_id"`
	Platform    string `gorm:"column:platform"`
	ChatID      string `gorm:"column:chat_id"`
	ThreadID    string `gorm:"column:thread_id"`
	UserID      string `gorm:"column:user_id"`
	AgentID     string `gorm:"column:agent_id"`
	IMChannelID string `gorm:"column:im_channel_id"`
}

func (testListSessionsIMChannelSession) TableName() string { return "im_channel_sessions" }
