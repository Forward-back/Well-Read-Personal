package dto

import (
	"testing"

	"gaze/internal/types"
)

func TestNewAuthLoginResponsePrefersUploadedAvatar(t *testing.T) {
	uploaded := "data:image/png;base64,ZmFrZQ=="
	resp := NewAuthLoginResponse(&types.LoginResponse{
		Success: true,
		User: &types.User{
			ID:         "user-1",
			Avatar:     "https://legacy.example/avatar.png",
			AvatarData: uploaded,
		},
	})

	if resp == nil || resp.User == nil {
		t.Fatal("expected a user in the login response")
	}
	if resp.User.Avatar != uploaded {
		t.Fatalf("expected uploaded avatar, got %q", resp.User.Avatar)
	}
}

func TestNewAuthLoginResponseFallsBackToLegacyAvatar(t *testing.T) {
	legacy := "https://legacy.example/avatar.png"
	resp := NewAuthLoginResponse(&types.LoginResponse{
		Success: true,
		User:    &types.User{ID: "user-1", Avatar: legacy},
	})

	if resp == nil || resp.User == nil {
		t.Fatal("expected a user in the login response")
	}
	if resp.User.Avatar != legacy {
		t.Fatalf("expected legacy avatar, got %q", resp.User.Avatar)
	}
}
