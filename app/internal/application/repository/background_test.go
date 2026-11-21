package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"gaze/internal/types"
)

type backgroundTestUser struct {
	ID               string `gorm:"type:varchar(36);primaryKey"`
	BackgroundChoice string `gorm:"column:background_choice;not null;default:'default'"`
	DeletedAt        gorm.DeletedAt
}

func (backgroundTestUser) TableName() string { return "users" }

func setupBackgroundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.UserBackground{}, &backgroundTestUser{}))
	return db
}

func TestBackgroundRepositoryScopesToOwner(t *testing.T) {
	db := setupBackgroundTestDB(t)
	repo := NewBackgroundRepository(db)
	ctx := context.Background()

	require.NoError(t, db.Create(&backgroundTestUser{ID: "user-1", BackgroundChoice: "default"}).Error)
	require.NoError(t, db.Create(&backgroundTestUser{ID: "user-2", BackgroundChoice: "default"}).Error)

	bg := &types.UserBackground{
		ID:          "bg-1",
		UserID:      "user-1",
		DisplayName: "photo.png",
		MimeType:    "image/png",
		SizeBytes:   12,
		FilePath:    "backgrounds/user-1/bg-1.png",
		CreatedAt:   time.Now(),
	}
	require.NoError(t, repo.CreateBackground(ctx, bg))

	items, err := repo.ListBackgrounds(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, items, 1)

	_, err = repo.GetBackground(ctx, "user-2", "bg-1")
	require.ErrorIs(t, err, ErrBackgroundNotFound)

	require.NoError(t, repo.UpdateBackgroundChoice(ctx, "user-1", "upload:bg-1"))
	choice, err := repo.GetBackgroundChoice(ctx, "user-1")
	require.NoError(t, err)
	require.Equal(t, "upload:bg-1", choice)

	require.NoError(t, repo.DeleteBackground(ctx, "user-1", "bg-1"))
	_, err = repo.GetBackground(ctx, "user-1", "bg-1")
	require.ErrorIs(t, err, ErrBackgroundNotFound)
}
