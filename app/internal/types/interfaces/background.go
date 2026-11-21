package interfaces

import (
	"context"
	"io"
	"mime/multipart"

	"gaze/internal/types"
)

// BackgroundService manages the owner's shell backgrounds: uploaded files,
// the selected background, and metadata. Built-in wallpapers are shipped with
// the frontend and only referenced by choice, not stored here.
type BackgroundService interface {
	ListBackgrounds(ctx context.Context, userID string) (*types.BackgroundList, error)
	UploadBackground(ctx context.Context, userID string, file *multipart.FileHeader) (*types.UserBackground, error)
	DeleteBackground(ctx context.Context, userID, id string) error
	SetBackgroundChoice(ctx context.Context, userID, choice string) error
	OpenBackground(ctx context.Context, userID, id string) (io.ReadCloser, *types.UserBackground, error)
}

// BackgroundRepository persists uploaded background metadata and the selected
// choice on the owner row.
type BackgroundRepository interface {
	CreateBackground(ctx context.Context, bg *types.UserBackground) error
	ListBackgrounds(ctx context.Context, userID string) ([]*types.UserBackground, error)
	GetBackground(ctx context.Context, userID, id string) (*types.UserBackground, error)
	DeleteBackground(ctx context.Context, userID, id string) error
	GetBackgroundChoice(ctx context.Context, userID string) (string, error)
	UpdateBackgroundChoice(ctx context.Context, userID, choice string) error
}
