package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

type stubReviewService struct {
	interfaces.ReviewService
	groups []*types.ReviewGroupNode
	card   *types.ReviewCard
}

func (s *stubReviewService) ListGroupTree(_ context.Context) ([]*types.ReviewGroupNode, error) {
	return s.groups, nil
}

func (s *stubReviewService) CreateCard(
	_ context.Context, req *types.CreateReviewCardRequest,
) (*types.ReviewCard, error) {
	if s.card != nil {
		return s.card, nil
	}
	return &types.ReviewCard{Question: req.Question, Answer: req.Answer}, nil
}

func TestReviewHandlerListGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/review/groups", nil)
	h := NewReviewHandler(&stubReviewService{
		groups: []*types.ReviewGroupNode{{Group: &types.ReviewGroup{ID: "g-1", Name: "默认卡组"}}},
	})

	h.ListGroups(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Success bool                     `json:"success"`
		Data    []*types.ReviewGroupNode `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "g-1", body.Data[0].Group.ID)
}

func TestReviewHandlerCreateCard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/review/cards",
		strings.NewReader(`{"question":"什么是栈？","answer":"后进先出。","sources":[{"knowledge_id":"k-1"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h := NewReviewHandler(&stubReviewService{})

	h.CreateCard(c)

	require.Equal(t, http.StatusCreated, w.Code)
	var body struct {
		Success bool              `json:"success"`
		Data    *types.ReviewCard `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.NotNil(t, body.Data)
	assert.Equal(t, "什么是栈？", body.Data.Question)
}

func TestReviewHandlerRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/review/cards", strings.NewReader(`{`))
	c.Request.Header.Set("Content-Type", "application/json")
	h := NewReviewHandler(&stubReviewService{})

	h.CreateCard(c)

	require.Len(t, c.Errors, 1)
}
