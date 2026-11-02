package service

import (
	"context"
	"errors"
	"testing"

	"gaze/internal/application/repository"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type failingSharedKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	err error
}

func (r *failingSharedKnowledgeRepo) GetKnowledgeBatch(context.Context, uint64, []string) ([]*types.Knowledge, error) {
	return []*types.Knowledge{{ID: "own", TenantID: 1, KnowledgeBaseID: "own-kb"}}, nil
}

func (r *failingSharedKnowledgeRepo) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	return nil, r.err
}

func newSharedAccessContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-1")
	return ctx
}

func TestSharedDocumentReadsPropagateStorageFailure(t *testing.T) {
	for _, failure := range []error{errors.New("database unavailable"), repository.ErrKnowledgeNotFound} {
		repo := &failingSharedKnowledgeRepo{err: failure}
		svc := &knowledgeService{repo: repo}
		search := &knowledgeBaseService{kgRepo: repo}
		rows, err := svc.GetKnowledgeBatchWithSharedAccess(newSharedAccessContext(), 1, []string{"own", "shared"})
		found, searchErr := search.fetchKnowledgeDataWithShared(newSharedAccessContext(), 1, []string{"own", "shared"})
		if errors.Is(failure, repository.ErrKnowledgeNotFound) {
			require.NoError(t, err)
			require.NoError(t, searchErr)
			require.Len(t, rows, 1)
			require.Len(t, found, 1)
		} else {
			require.ErrorIs(t, err, failure)
			require.ErrorIs(t, searchErr, failure)
			require.Nil(t, rows)
			require.Nil(t, found)
		}
	}
}
