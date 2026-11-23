package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gaze/internal/logger"
	"gaze/internal/types"
)

// maybeMigrateLegacyRun upgrades a finished legacy run from the old notes JSON
// to the structured four-table state. It is safe to call on every Get: the
// deterministic IDs plus ON CONFLICT DO NOTHING make it idempotent. Active or
// stopped legacy runs stay on the old engine until they finish.
func (s *readingRunService) maybeMigrateLegacyRun(ctx context.Context, run *types.ReadingRun) error {
	if run == nil || run.Status != types.ReadingRunFinished {
		return nil
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil
	}
	if plan.StateVersion >= readingStructuredStateVersion {
		return nil
	}
	return s.migrateLegacyRun(ctx, run)
}

// MigrateLegacyFinishedRuns is the ops batch trigger. It migrates at most
// limit finished legacy runs and returns how many were processed.
func (s *readingRunService) MigrateLegacyFinishedRuns(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	runs, err := s.repo.ListLegacyFinishedRuns(ctx, limit)
	if err != nil {
		return 0, err
	}
	migrated := 0
	for _, run := range runs {
		if err := s.migrateLegacyRun(ctx, run); err != nil {
			logger.Warnf(ctx, "reading: migrate legacy run %s failed: %v", run.ID, err)
			continue
		}
		migrated++
	}
	return migrated, nil
}

func (s *readingRunService) migrateLegacyRun(ctx context.Context, run *types.ReadingRun) error {
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return err
	}
	var notes types.ReadingNotes
	_ = json.Unmarshal(run.Notes, &notes)
	final := map[string]any{}
	_ = json.Unmarshal(run.Final, &final)
	now := time.Now()
	legacyID := func(kind string, index int) string {
		return uuid.NewSHA1(
			uuid.NameSpaceOID,
			[]byte(fmt.Sprintf("reading-legacy:%s:%s:%d", run.ID, kind, index)),
		).String()
	}

	conclusions := make([]*types.ReadingConclusion, 0, len(notes.Confirmed)+len(notes.Decisions))
	seenConclusions := map[string]struct{}{}
	appendConclusion := func(text, source string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		key := normalizeReadingText(text)
		if _, ok := seenConclusions[key]; ok {
			return
		}
		seenConclusions[key] = struct{}{}
		gates := types.ReadingConclusionGates{
			ReferencesOK: false, ConflictFree: true, CrossDocConsistent: true,
			UserAdjudicated: source == "user",
		}
		rawGates, _ := json.Marshal(gates)
		conclusions = append(conclusions, &types.ReadingConclusion{
			ID: legacyID("conclusion", len(conclusions)), RunID: run.ID, TenantID: run.TenantID,
			Text: text, Evidence: types.JSON("[]"), Status: types.ReadingConclusionConfirmed,
			Source: source, Gates: types.JSON(rawGates), CreatedAt: now, UpdatedAt: now,
		})
	}
	for _, text := range notes.Decisions {
		appendConclusion(text, "user")
	}
	for _, text := range notes.Confirmed {
		appendConclusion(text, types.ReadingConclusionSourceLegacy)
	}

	terms := make([]*types.ReadingTerm, 0, len(notes.Terms))
	seenTerms := map[string]struct{}{}
	for _, text := range notes.Terms {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		term, definition := text, "（历史笔记，无结构化定义）"
		index := strings.Index(text, "：")
		sepLen := len("：")
		if index < 0 {
			index = strings.Index(text, ":")
			sepLen = 1
		}
		if index > 0 {
			term = strings.TrimSpace(text[:index])
			definition = strings.TrimSpace(text[index+sepLen:])
			if definition == "" {
				definition = "（历史笔记，无结构化定义）"
			}
		}
		key := normalizeReadingText(term)
		if _, ok := seenTerms[key]; ok {
			continue
		}
		seenTerms[key] = struct{}{}
		terms = append(terms, &types.ReadingTerm{
			ID: legacyID("term", len(terms)), RunID: run.ID, TenantID: run.TenantID,
			Term: term, Definition: definition, Source: types.JSON("[]"),
			Canonical: false, Status: types.ReadingTermCandidate, CreatedAt: now, UpdatedAt: now,
		})
	}

	questions := make([]*types.ReadingQuestionNode, 0, len(notes.OpenQuestions))
	seenQuestions := map[string]struct{}{}
	for _, text := range notes.OpenQuestions {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		key := normalizeReadingText(text)
		if _, ok := seenQuestions[key]; ok {
			continue
		}
		seenQuestions[key] = struct{}{}
		questions = append(questions, &types.ReadingQuestionNode{
			ID: legacyID("question", len(questions)), RunID: run.ID, TenantID: run.TenantID,
			Text: text, Why: "历史笔记迁移，无结构化出处",
			Options: types.JSON("[]"), Evidence: types.JSON("[]"), Status: types.ReadingQuestionPending,
			CreatedAt: now, UpdatedAt: now,
		})
	}

	contradictions := make([]*types.ReadingContradiction, 0, len(notes.Contradictions))
	seenContradictions := map[string]struct{}{}
	for _, text := range notes.Contradictions {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		key := normalizeReadingText(text)
		if _, ok := seenContradictions[key]; ok {
			continue
		}
		seenContradictions[key] = struct{}{}
		contradictions = append(contradictions, &types.ReadingContradiction{
			ID: legacyID("contradiction", len(contradictions)), RunID: run.ID, TenantID: run.TenantID,
			Description: "[历史笔记] " + text, EvidenceA: types.JSON("[]"), EvidenceB: types.JSON("[]"),
			Status: types.ReadingContradictionOpen, CreatedAt: now, UpdatedAt: now,
		})
	}

	plan.StateVersion = readingStructuredStateVersion
	planJSON, _ := json.Marshal(plan)
	run.Plan = types.JSON(planJSON)
	final["legacy_migrated"] = true
	finalJSON, _ := json.Marshal(final)
	run.Final = types.JSON(finalJSON)
	run.UpdatedAt = now
	return s.repo.MigrateLegacyRun(ctx, run, questions, conclusions, terms, contradictions)
}
