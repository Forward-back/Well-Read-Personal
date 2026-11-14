package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEngineSyncTrigger struct {
	count int
	err   error
	calls int
}

func (f *fakeEngineSyncTrigger) TriggerKBSync(_ context.Context, _ string, _ []string) (int, error) {
	f.calls++
	return f.count, f.err
}

func newEngineSyncContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/knowledge-bases/kb-1/engine-sync", nil)
	c.Params = gin.Params{{Key: "id", Value: "kb-1"}}
	return c, w
}

func TestTriggerEngineSyncReturnsQueuedCount(t *testing.T) {
	trigger := &fakeEngineSyncTrigger{count: 2}
	h := &KnowledgeBaseHandler{engineSync: trigger}
	c, w := newEngineSyncContext()

	h.TriggerEngineSync(c)

	require.Equal(t, 1, trigger.calls)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Queued int `json:"queued"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 2, body.Data.Queued)
}

func TestTriggerEngineSyncPropagatesServiceError(t *testing.T) {
	trigger := &fakeEngineSyncTrigger{err: errors.New("kb missing")}
	h := &KnowledgeBaseHandler{engineSync: trigger}
	c, _ := newEngineSyncContext()

	h.TriggerEngineSync(c)

	require.Len(t, c.Errors, 1)
	assert.Contains(t, c.Errors[0].Error(), "kb missing")
}

func TestTriggerEngineSyncRejectsMissingService(t *testing.T) {
	h := &KnowledgeBaseHandler{}
	c, _ := newEngineSyncContext()

	h.TriggerEngineSync(c)

	require.Len(t, c.Errors, 1)
	assert.Contains(t, c.Errors[0].Error(), "引擎同步服务未配置")
}
