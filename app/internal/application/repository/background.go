package repository

import (
	"context"
	"errors"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrBackgroundNotFound = errors.New("background not found")

type backgroundRepository struct {
	db *gorm.DB
}

func NewBackgroundRepository(db *gorm.DB) interfaces.BackgroundRepository {
	return &backgroundRepository{db: db}
}

func (r *backgroundRepository) CreateBackground(ctx context.Context, bg *types.UserBackground) error {
	return r.db.WithContext(ctx).Create(bg).Error
}

func (r *backgroundRepository) ListBackgrounds(ctx context.Context, userID string) ([]*types.UserBackground, error) {
	var items []*types.UserBackground
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Find(&items).Error
	return items, err
}

func (r *backgroundRepository) GetBackground(ctx context.Context, userID, id string) (*types.UserBackground, error) {
	var bg types.UserBackground
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&bg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBackgroundNotFound
	}
	if err != nil {
		return nil, err
	}
	return &bg, nil
}

func (r *backgroundRepository) DeleteBackground(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&types.UserBackground{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrBackgroundNotFound
	}
	return nil
}

func (r *backgroundRepository) GetBackgroundChoice(ctx context.Context, userID string) (string, error) {
	var choice string
	err := r.db.WithContext(ctx).
		Model(&types.User{}).
		Select("background_choice").
		Where("id = ?", userID).
		Scan(&choice).Error
	if err != nil {
		return "", err
	}
	return choice, nil
}

func (r *backgroundRepository) UpdateBackgroundChoice(ctx context.Context, userID, choice string) error {
	res := r.db.WithContext(ctx).
		Model(&types.User{}).
		Where("id = ?", userID).
		UpdateColumn("background_choice", choice)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}
