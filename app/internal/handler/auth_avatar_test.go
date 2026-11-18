package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

const testAvatarPNGDataURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZQmcAAAAASUVORK5CYII="

type stubAvatarUserService struct {
	interfaces.UserService
	user   *types.User
	avatar string
}

func (s *stubAvatarUserService) GetCurrentUser(context.Context) (*types.User, error) {
	return s.user, nil
}

func (s *stubAvatarUserService) UpdateUserAvatar(_ context.Context, _ string, avatar string) error {
	s.avatar = avatar
	return nil
}

func newAvatarTestRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPCode, gin.H{"success": false, "error": gin.H{"code": appErr.Code, "message": appErr.Message}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	})
	r.PUT("/auth/me/avatar", h.UpdateAvatar)
	return r
}

func doUpdateAvatar(t *testing.T, r *gin.Engine, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/auth/me/avatar", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateAvatar_StoresValidPNGDataURL(t *testing.T) {
	svc := &stubAvatarUserService{user: &types.User{ID: "user-1", Email: "owner@example.com"}}
	h := &AuthHandler{userService: svc}

	w := doUpdateAvatar(t, newAvatarTestRouter(h), map[string]string{"avatar": testAvatarPNGDataURL})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.avatar != testAvatarPNGDataURL {
		t.Fatalf("avatar not stored: %q", svc.avatar)
	}
}

func TestUpdateAvatar_ClearsWithEmptyString(t *testing.T) {
	svc := &stubAvatarUserService{user: &types.User{ID: "user-1", Email: "owner@example.com"}, avatar: "stale"}
	h := &AuthHandler{userService: svc}

	w := doUpdateAvatar(t, newAvatarTestRouter(h), map[string]string{"avatar": ""})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if svc.avatar != "" {
		t.Fatalf("avatar should be cleared, got %q", svc.avatar)
	}
}

func TestUpdateAvatar_RejectsUnsupportedFormat(t *testing.T) {
	h := &AuthHandler{userService: &stubAvatarUserService{user: &types.User{ID: "user-1"}}}

	w := doUpdateAvatar(t, newAvatarTestRouter(h), map[string]string{
		"avatar": "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAvatar_RejectsMislabelledContent(t *testing.T) {
	h := &AuthHandler{userService: &stubAvatarUserService{user: &types.User{ID: "user-1"}}}

	w := doUpdateAvatar(t, newAvatarTestRouter(h), map[string]string{
		"avatar": "data:image/png;base64,bm90LWFuLWltYWdl",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
