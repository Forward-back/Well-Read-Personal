package repository

import (
	"context"

	"gaze/internal/types"
)

func (r *readingRunRepository) CreateFeynmanAttempt(
	ctx context.Context, attempt *types.ReadingFeynmanAttempt,
) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *readingRunRepository) ListFeynmanAttempts(
	ctx context.Context, runID, sectionID string,
) ([]*types.ReadingFeynmanAttempt, error) {
	query := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at DESC")
	if sectionID != "" {
		query = query.Where("section_id = ?", sectionID)
	}
	var attempts []*types.ReadingFeynmanAttempt
	if err := query.Find(&attempts).Error; err != nil {
		return nil, err
	}
	return attempts, nil
}

func (r *readingRunRepository) CreateAnnotation(
	ctx context.Context, annotation *types.ReadingAnnotation,
) error {
	return r.db.WithContext(ctx).Create(annotation).Error
}

func (r *readingRunRepository) ListAnnotations(
	ctx context.Context, runID string,
) ([]*types.ReadingAnnotation, error) {
	var annotations []*types.ReadingAnnotation
	if err := r.db.WithContext(ctx).
		Where("run_id = ? AND status = ?", runID, "active").
		Order("created_at ASC").
		Find(&annotations).Error; err != nil {
		return nil, err
	}
	return annotations, nil
}

func (r *readingRunRepository) GetAnnotation(
	ctx context.Context, runID, annotationID string,
) (*types.ReadingAnnotation, error) {
	var annotation types.ReadingAnnotation
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND id = ? AND status = ?", runID, annotationID, "active").
		First(&annotation).Error
	if err != nil {
		return nil, err
	}
	return &annotation, nil
}

func (r *readingRunRepository) DeleteAnnotation(
	ctx context.Context, runID, annotationID string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingAnnotation{}).
		Where("run_id = ? AND id = ?", runID, annotationID).
		Update("status", "deleted").Error
}
