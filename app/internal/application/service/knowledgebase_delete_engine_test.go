package service

import (
	"context"
	"errors"
	"testing"

	"gaze/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingDeleteEngine struct {
	engine.RAGEngine
	deleted []string
	errByID map[string]error
}

func (e *recordingDeleteEngine) DeleteDocument(_ context.Context, documentID string) error {
	e.deleted = append(e.deleted, documentID)
	if e.errByID != nil {
		return e.errByID[documentID]
	}
	return nil
}

func TestDeleteKnowledgeFromEngineRemovesEveryDocument(t *testing.T) {
	eng := &recordingDeleteEngine{}
	svc := &knowledgeBaseService{ragEngine: eng}

	svc.deleteKnowledgeFromEngine(context.Background(), []string{"k-1", "k-2"})

	assert.Equal(t, []string{"k-1", "k-2"}, eng.deleted)
}

func TestDeleteKnowledgeFromEngineContinuesAfterFailure(t *testing.T) {
	eng := &recordingDeleteEngine{
		errByID: map[string]error{"k-1": errors.New("engine unavailable")},
	}
	svc := &knowledgeBaseService{ragEngine: eng}

	svc.deleteKnowledgeFromEngine(context.Background(), []string{"k-1", "k-2"})

	require.Equal(t, []string{"k-1", "k-2"}, eng.deleted)
}

func TestDeleteKnowledgeFromEngineIsNilSafe(t *testing.T) {
	svc := &knowledgeBaseService{}
	svc.deleteKnowledgeFromEngine(context.Background(), nil)
	svc.deleteKnowledgeFromEngine(context.Background(), []string{"k-1"})
}
