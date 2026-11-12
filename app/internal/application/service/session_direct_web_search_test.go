package service

import (
	"context"
	"testing"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingWebSearchService struct {
	interfaces.WebSearchService
	providerID string
	config     *types.WebSearchConfig
}

func (s *recordingWebSearchService) Search(
	_ context.Context, providerID string, config *types.WebSearchConfig, _ string,
) ([]*types.WebSearchResult, error) {
	s.providerID = providerID
	s.config = config
	return nil, nil
}

type recordingDefaultProviderRepo struct {
	interfaces.WebSearchProviderRepository
	called bool
}

func (r *recordingDefaultProviderRepo) GetDefault(
	context.Context, uint64,
) (*types.WebSearchProviderEntity, error) {
	r.called = true
	return &types.WebSearchProviderEntity{ID: "entity-default"}, nil
}

func TestDirectWebSearchReadsTenantKVOnly(t *testing.T) {
	search := &recordingWebSearchService{}
	providerRepo := &recordingDefaultProviderRepo{}
	svc := &sessionService{
		webSearchService:      search,
		webSearchProviderRepo: providerRepo,
	}
	tenant := &types.Tenant{
		WebSearchConfig: &types.WebSearchConfig{
			Provider:   "deepseek",
			APIKey:     "tenant-kv-key",
			MaxResults: 5,
		},
	}

	_, err := svc.directWebSearch(context.Background(), tenant, "query")
	require.NoError(t, err)
	assert.False(t, providerRepo.called, "direct mode must not consult provider entities")
	assert.Empty(t, search.providerID)
	require.NotNil(t, search.config)
	assert.Equal(t, "tenant-kv-key", search.config.APIKey)
	assert.Equal(t, "deepseek", search.config.Provider)
}
