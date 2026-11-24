package repository

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

type readingRunRepository struct{ db *gorm.DB }

func NewReadingRunRepository(db *gorm.DB) interfaces.ReadingRunRepository {
	return &readingRunRepository{db: db}
}

func (r *readingRunRepository) CreateRun(ctx context.Context, run *types.ReadingRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *readingRunRepository) GetRun(
	ctx context.Context, tenantID uint64, userID, runID string,
) (*types.ReadingRun, error) {
	var run types.ReadingRun
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, runID).
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *readingRunRepository) GetActiveRun(
	ctx context.Context, tenantID uint64, userID, sessionID string,
) (*types.ReadingRun, error) {
	var run types.ReadingRun
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND session_id = ? AND status IN ?",
			tenantID, userID, sessionID,
			[]types.ReadingRunStatus{
				types.ReadingRunPlanning, types.ReadingRunReading, types.ReadingRunWaitingUser,
			}).
		Order("created_at DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *readingRunRepository) UpdateRun(ctx context.Context, run *types.ReadingRun) error {
	return updateReadingRun(r.db.WithContext(ctx), run)
}

func updateReadingRun(db *gorm.DB, run *types.ReadingRun) error {
	return db.
		Model(&types.ReadingRun{}).
		Where("tenant_id = ? AND user_id = ? AND id = ?", run.TenantID, run.UserID, run.ID).
		Updates(map[string]any{
			"document_ids":        run.DocumentIDs,
			"skill_id":            run.SkillID,
			"reading_model_id":    run.ReadingModelID,
			"extraction_model_id": run.ExtractionModelID,
			"synthesis_model_id":  run.SynthesisModelID,
			"phase":               run.Phase,
			"status":              run.Status,
			"plan":                run.Plan,
			"notes":               run.Notes,
			"final":               run.Final,
			"current_section_id":  run.CurrentSectionID,
			"step_count":          run.StepCount,
			"token_usage":         run.TokenUsage,
			"updated_at":          run.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) StopActiveRuns(
	ctx context.Context, tenantID uint64, userID, sessionID string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingRun{}).
		Where("tenant_id = ? AND user_id = ? AND session_id = ? AND status IN ?",
			tenantID, userID, sessionID,
			[]types.ReadingRunStatus{
				types.ReadingRunPlanning, types.ReadingRunReading, types.ReadingRunWaitingUser,
			}).
		Updates(map[string]any{
			"status":     types.ReadingRunStopped,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}).Error
}

func (r *readingRunRepository) CreateStep(ctx context.Context, step *types.ReadingRunStep) error {
	return r.db.WithContext(ctx).Create(step).Error
}

func (r *readingRunRepository) ListSteps(ctx context.Context, runID string) ([]*types.ReadingRunStep, error) {
	var steps []*types.ReadingRunStep
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("seq ASC").
		Find(&steps).Error; err != nil {
		return nil, err
	}
	return steps, nil
}

func (r *readingRunRepository) UpdateStep(ctx context.Context, step *types.ReadingRunStep) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingRunStep{}).
		Where("id = ?", step.ID).
		Updates(map[string]any{
			"question":      step.Question,
			"user_feedback": step.UserFeedback,
			"notes_patch":   step.NotesPatch,
			"citations":     step.Citations,
			"status":        step.Status,
		}).Error
}

func (r *readingRunRepository) ListQuestions(
	ctx context.Context, runID string,
) ([]*types.ReadingQuestionNode, error) {
	var rows []*types.ReadingQuestionNode
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("round ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) UpdateQuestion(
	ctx context.Context, question *types.ReadingQuestionNode,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingQuestionNode{}).
		Where("id = ? AND run_id = ?", question.ID, question.RunID).
		Updates(map[string]any{
			"options":         question.Options,
			"evidence":        question.Evidence,
			"answer":          question.Answer,
			"answer_source":   question.AnswerSource,
			"judgment":        question.Judgment,
			"judgment_reason": question.JudgmentReason,
			"judged_at":       question.JudgedAt,
			"status":          question.Status,
			"updated_at":      question.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) ListConclusions(
	ctx context.Context, runID string,
) ([]*types.ReadingConclusion, error) {
	var rows []*types.ReadingConclusion
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("round ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) UpdateConclusion(
	ctx context.Context, conclusion *types.ReadingConclusion,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingConclusion{}).
		Where("id = ? AND run_id = ?", conclusion.ID, conclusion.RunID).
		Updates(map[string]any{
			"text":       conclusion.Text,
			"status":     conclusion.Status,
			"gates":      conclusion.Gates,
			"updated_at": conclusion.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) ListTerms(
	ctx context.Context, runID string,
) ([]*types.ReadingTerm, error) {
	var rows []*types.ReadingTerm
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) UpdateTerm(
	ctx context.Context, term *types.ReadingTerm,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingTerm{}).
		Where("id = ? AND run_id = ?", term.ID, term.RunID).
		Updates(map[string]any{
			"definition": term.Definition,
			"status":     term.Status,
			"updated_at": term.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) ListUnderstandings(
	ctx context.Context, runID string,
) ([]*types.ReadingUnderstanding, error) {
	var rows []*types.ReadingUnderstanding
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("round ASC, created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) SaveUnderstanding(
	ctx context.Context, understanding *types.ReadingUnderstanding,
) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "run_id"}, {Name: "section_id"}, {Name: "round"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"text", "decision", "correction", "updated_at",
			}),
		}).
		Create(understanding).Error
}

func (r *readingRunRepository) UpdateUnderstanding(
	ctx context.Context, understanding *types.ReadingUnderstanding,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingUnderstanding{}).
		Where("id = ? AND run_id = ?", understanding.ID, understanding.RunID).
		Updates(map[string]any{
			"decision":   understanding.Decision,
			"correction": understanding.Correction,
			"updated_at": understanding.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) GetMemory(
	ctx context.Context, runID string,
) (*types.ReadingRunMemory, error) {
	var memory types.ReadingRunMemory
	err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&memory).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &memory, nil
}

func (r *readingRunRepository) SaveMemory(
	ctx context.Context, memory *types.ReadingRunMemory,
) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "run_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"knowledge", "user_model", "task", "updated_at",
			}),
		}).
		Create(memory).Error
}

func (r *readingRunRepository) ListSelfTests(
	ctx context.Context, runID string,
) ([]*types.ReadingSelfTest, error) {
	var rows []*types.ReadingSelfTest
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) CreateSelfTests(
	ctx context.Context, tests []*types.ReadingSelfTest,
) (int, error) {
	if len(tests) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(tests)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func (r *readingRunRepository) UpdateSelfTestStatus(
	ctx context.Context, runID, selfTestID, status string,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&types.ReadingSelfTest{}).
		Where("id = ? AND run_id = ? AND status = ?",
			selfTestID, runID, types.ReadingSelfTestCandidate).
		Updates(map[string]any{
			"status":     status,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *readingRunRepository) ListDrafts(
	ctx context.Context, tenantID uint64, userID string,
) ([]*types.ReadingDraft, error) {
	var rows []*types.ReadingDraft
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("updated_at DESC").
		Limit(20).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) GetDraft(
	ctx context.Context, tenantID uint64, userID, draftID string,
) (*types.ReadingDraft, error) {
	var draft types.ReadingDraft
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, draftID).
		First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r *readingRunRepository) CreateDraft(
	ctx context.Context, draft *types.ReadingDraft,
) error {
	return r.db.WithContext(ctx).Create(draft).Error
}

func (r *readingRunRepository) UpdateDraft(
	ctx context.Context, draft *types.ReadingDraft,
) error {
	return r.db.WithContext(ctx).
		Model(&types.ReadingDraft{}).
		Where("tenant_id = ? AND user_id = ? AND id = ?", draft.TenantID, draft.UserID, draft.ID).
		Updates(map[string]any{
			"name":         draft.Name,
			"document_ids": draft.DocumentIDs,
			"depth":        draft.Depth,
			"goal":         draft.Goal,
			"updated_at":   draft.UpdatedAt,
		}).Error
}

func (r *readingRunRepository) DeleteDraft(
	ctx context.Context, tenantID uint64, userID, draftID string,
) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND id = ?", tenantID, userID, draftID).
		Delete(&types.ReadingDraft{}).Error
}

func (r *readingRunRepository) ListContradictions(
	ctx context.Context, runID string,
) ([]*types.ReadingContradiction, error) {
	var rows []*types.ReadingContradiction
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *readingRunRepository) SaveRound(
	ctx context.Context, run *types.ReadingRun,
	questions []*types.ReadingQuestionNode,
	conclusions []*types.ReadingConclusion,
	terms []*types.ReadingTerm,
	contradictions []*types.ReadingContradiction,
	step *types.ReadingRunStep,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateReadingRun(tx, run); err != nil {
			return err
		}
		if len(questions) > 0 {
			if err := tx.Create(questions).Error; err != nil {
				return err
			}
		}
		if len(conclusions) > 0 {
			if err := tx.Create(conclusions).Error; err != nil {
				return err
			}
		}
		if len(terms) > 0 {
			if err := tx.Create(terms).Error; err != nil {
				return err
			}
		}
		if len(contradictions) > 0 {
			if err := tx.Create(contradictions).Error; err != nil {
				return err
			}
		}
		if step != nil {
			if err := tx.Create(step).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *readingRunRepository) SaveFeedback(
	ctx context.Context, run *types.ReadingRun,
	question *types.ReadingQuestionNode,
	conclusion *types.ReadingConclusion,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateReadingRun(tx, run); err != nil {
			return err
		}
		if question != nil {
			if err := tx.Model(&types.ReadingQuestionNode{}).
				Where("id = ?", question.ID).
				Updates(map[string]any{
					"answer":          question.Answer,
					"answer_source":   question.AnswerSource,
					"judgment":        question.Judgment,
					"judgment_reason": question.JudgmentReason,
					"judged_at":       question.JudgedAt,
					"status":          question.Status,
					"updated_at":      question.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if conclusion != nil {
			if err := tx.Create(conclusion).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *readingRunRepository) ResolveContradiction(
	ctx context.Context,
	contradiction *types.ReadingContradiction,
	conclusion *types.ReadingConclusion,
	updatedConclusions []*types.ReadingConclusion,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if contradiction != nil {
			if err := tx.Model(&types.ReadingContradiction{}).
				Where("id = ?", contradiction.ID).
				Updates(map[string]any{
					"status":     contradiction.Status,
					"resolution": contradiction.Resolution,
					"updated_at": contradiction.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}
		if conclusion != nil {
			if err := tx.Create(conclusion).Error; err != nil {
				return err
			}
		}
		for _, item := range updatedConclusions {
			if item == nil {
				continue
			}
			if err := tx.Model(&types.ReadingConclusion{}).
				Where("id = ?", item.ID).
				Updates(map[string]any{
					"status":     item.Status,
					"gates":      item.Gates,
					"updated_at": item.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *readingRunRepository) GetReportDraft(
	ctx context.Context, tenantID uint64, userID, runID string,
) (*types.ReadingReportDraft, error) {
	var draft types.ReadingReportDraft
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND run_id = ?", tenantID, userID, runID).
		First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

func (r *readingRunRepository) SaveReportDraft(
	ctx context.Context, draft *types.ReadingReportDraft, baseVersion int,
) (bool, error) {
	if baseVersion <= 0 {
		draft.Version = 1
		result := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(draft)
		if result.Error != nil {
			return false, result.Error
		}
		return result.RowsAffected == 1, nil
	}
	result := r.db.WithContext(ctx).
		Model(&types.ReadingReportDraft{}).
		Where("run_id = ? AND tenant_id = ? AND user_id = ? AND version = ?",
			draft.RunID, draft.TenantID, draft.UserID, baseVersion).
		Updates(map[string]any{
			"content_md": draft.ContentMD,
			"version":    gorm.Expr("version + 1"),
			"updated_at": draft.UpdatedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		draft.Version = baseVersion + 1
		return true, nil
	}
	return false, nil
}

func (r *readingRunRepository) DeleteReportDraft(
	ctx context.Context, tenantID uint64, userID, runID string, version int,
) (bool, error) {
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND run_id = ?", tenantID, userID, runID)
	if version >= 0 {
		query = query.Where("version = ?", version)
	}
	result := query.Delete(&types.ReadingReportDraft{})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 || version < 0 {
		return true, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&types.ReadingReportDraft{}).
		Where("tenant_id = ? AND user_id = ? AND run_id = ?", tenantID, userID, runID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *readingRunRepository) ListLegacyFinishedRuns(
	ctx context.Context, limit int,
) ([]*types.ReadingRun, error) {
	query := r.db.WithContext(ctx).
		Where("status = ?", types.ReadingRunFinished).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var runs []*types.ReadingRun
	if err := query.Find(&runs).Error; err != nil {
		return nil, err
	}
	// Filter in Go so PG and SQLite share the same code path; plan is JSONB in
	// PG and TEXT in SQLite.
	out := runs[:0]
	for _, run := range runs {
		var plan types.ReadingPlan
		if err := json.Unmarshal(run.Plan, &plan); err != nil {
			continue
		}
		if plan.StateVersion < 2 {
			out = append(out, run)
		}
	}
	return out, nil
}

func (r *readingRunRepository) MigrateLegacyRun(
	ctx context.Context, run *types.ReadingRun,
	questions []*types.ReadingQuestionNode,
	conclusions []*types.ReadingConclusion,
	terms []*types.ReadingTerm,
	contradictions []*types.ReadingContradiction,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateReadingRun(tx, run); err != nil {
			return err
		}
		insert := func(values any) error {
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(values).Error
		}
		if len(questions) > 0 {
			if err := insert(questions); err != nil {
				return err
			}
		}
		if len(conclusions) > 0 {
			if err := insert(conclusions); err != nil {
				return err
			}
		}
		if len(terms) > 0 {
			if err := insert(terms); err != nil {
				return err
			}
		}
		if len(contradictions) > 0 {
			if err := insert(contradictions); err != nil {
				return err
			}
		}
		return nil
	})
}
