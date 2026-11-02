package repository

import (
	"context"
	"errors"
	"time"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type resourceRepository struct{ db *gorm.DB }

// NewResourceRepository creates the persistence adapter for resource metadata.
func NewResourceRepository(db *gorm.DB) interfaces.ResourceRepository {
	return &resourceRepository{db: db}
}

func (r *resourceRepository) Create(ctx context.Context, resource *types.StoredResource) error {
	return r.db.WithContext(ctx).Create(resource).Error
}

func (r *resourceRepository) GetByID(ctx context.Context, id string) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).Where("id = ? AND state = ?", id, types.ResourceStateActive).First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) GetByHandle(ctx context.Context, handle string) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).
		Where("handle = ? AND state = ?", handle, types.ResourceStateActive).
		First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) GetByTenantLocation(
	ctx context.Context,
	tenantID uint64,
	locationHash string,
) (*types.StoredResource, error) {
	var resource types.StoredResource
	err := r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND location_hash = ? AND state = ?",
			tenantID,
			locationHash,
			types.ResourceStateActive,
		).
		First(&resource).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &resource, err
}

func (r *resourceRepository) MarkDeleted(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&types.StoredResource{}).Where("id = ?", id).
		Updates(map[string]interface{}{"state": types.ResourceStateDeleted, "deleted_at": time.Now()}).Error
}

func (r *resourceRepository) CreateBinding(ctx context.Context, binding *types.ResourceBinding) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(binding).Error
}

// DeleteBinding removes one owner's claim on a resource. Deleting a claim that
// was never recorded is not an error: callers release optimistically, from a
// content scan that cannot know which references were bound.
func (r *resourceRepository) DeleteBinding(ctx context.Context, resourceID, ownerType, ownerID string) error {
	return r.db.WithContext(ctx).
		Where("resource_id = ? AND owner_type = ? AND owner_id = ?", resourceID, ownerType, ownerID).
		Delete(&types.ResourceBinding{}).Error
}

func (r *resourceRepository) CountBindings(ctx context.Context, resourceID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.ResourceBinding{}).
		Where("resource_id = ?", resourceID).
		Count(&count).Error
	return count, err
}
