package service

import (
	"context"
	"testing"

	"gaze/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtinModelContext(systemAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	return context.WithValue(ctx, types.SystemAdminContextKey, systemAdmin)
}

// The personal edition has one owner who also manages built-in models; no
// system-administrator flag is required.
func TestUpdateBuiltinModel_OwnerAllowed(t *testing.T) {
	stored := &types.Model{
		ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
		ManagedBy: types.BuiltinModelManagedBy,
	}
	var saved *types.Model
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(model *types.Model) error {
			copy := *model
			saved = &copy
			return nil
		},
	}, nil, nil, nil, nil, nil)

	require.NoError(t, svc.UpdateModel(builtinModelContext(false), &types.Model{ID: stored.ID, Name: "edited"}))
	require.NotNil(t, saved)
	assert.Equal(t, uint64(10000), saved.TenantID)
	assert.True(t, saved.IsBuiltin)
	assert.Empty(t, saved.ManagedBy, "UI edit must stop later YAML reconciliation")
}

func TestUpdateBuiltinModel_SystemAdminCreatesRuntimeOverride(t *testing.T) {
	stored := &types.Model{
		ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
		ManagedBy: types.BuiltinModelManagedBy,
	}
	var saved *types.Model
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(model *types.Model) error {
			copy := *model
			saved = &copy
			return nil
		},
	}, nil, nil, nil, nil, nil)

	input := &types.Model{ID: stored.ID, Name: "edited"}
	require.NoError(t, svc.UpdateModel(builtinModelContext(true), input))
	require.NotNil(t, saved)
	assert.Equal(t, uint64(10000), saved.TenantID)
	assert.True(t, saved.IsBuiltin)
	assert.Empty(t, saved.ManagedBy, "UI edit must stop later YAML reconciliation")
}

func TestUpdateBuiltinModelCredentials_OwnerAllowed(t *testing.T) {
	newKey := "sk-new"

	t.Run("owner saves runtime override", func(t *testing.T) {
		stored := &types.Model{
			ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
			ManagedBy: types.BuiltinModelManagedBy,
		}
		var saved *types.Model
		svc := NewModelService(&stubModelRepoForDelete{
			model: stored,
			update: func(model *types.Model) error {
				copy := *model
				saved = &copy
				return nil
			},
		}, nil, nil, nil, nil, nil)

		updated, err := svc.UpdateModelCredentials(
			builtinModelContext(false), stored.ID, &newKey, nil,
		)
		require.NoError(t, err)
		// The secret is persisted on the model for provider calls; response
		// DTOs mask it and only expose configured=false/true metadata.
		assert.Equal(t, newKey, updated.Parameters.APIKey)
		require.NotNil(t, saved)
		assert.Equal(t, uint64(10000), saved.TenantID)
		assert.True(t, saved.IsBuiltin)
		assert.Empty(t, saved.ManagedBy)
	})

	t.Run("unchanged credential keeps YAML ownership", func(t *testing.T) {
		stored := &types.Model{
			ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
			ManagedBy:  types.BuiltinModelManagedBy,
			Parameters: types.ModelParameters{APIKey: newKey},
		}
		updates := 0
		svc := NewModelService(&stubModelRepoForDelete{
			model: stored,
			update: func(*types.Model) error {
				updates++
				return nil
			},
		}, nil, nil, nil, nil, nil)

		updated, err := svc.UpdateModelCredentials(
			builtinModelContext(false), stored.ID, &newKey, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, newKey, updated.Parameters.APIKey)
		assert.Zero(t, updates)
		assert.Equal(t, types.BuiltinModelManagedBy, updated.ManagedBy)
	})
}
