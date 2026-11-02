package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

// ResolveContradiction records a user adjudication of a contradiction. When the
// user picks A or B, conclusions supported by that side are promoted back to
// confirmed; the other side stays contradicted. The decision itself is stored
// as a user-sourced conclusion so it appears in the report's decision section.
func (s *readingRunService) ResolveContradiction(
	ctx context.Context, runID, contradictionID string,
	req *types.ReadingContradictionResolutionRequest,
) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil || run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if plan.StateVersion < readingStructuredStateVersion {
		return nil, werrors.NewValidationError("仅结构化精读支持矛盾裁定")
	}
	contradictions, err := s.repo.ListContradictions(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	var target *types.ReadingContradiction
	for _, item := range contradictions {
		if item.ID == strings.TrimSpace(contradictionID) {
			target = item
			break
		}
	}
	if target == nil {
		return nil, werrors.NewNotFoundError("矛盾不存在")
	}
	if target.Status == types.ReadingContradictionClosed {
		return nil, werrors.NewValidationError("矛盾已关闭，不能再次裁定")
	}
	choice := strings.ToLower(strings.TrimSpace(req.Choice))
	note := strings.TrimSpace(req.Resolution)
	if choice != "a" && choice != "b" && choice != "custom" {
		if note == "" {
			return nil, werrors.NewValidationError("请选择裁定方向或填写裁定说明")
		}
		choice = "custom"
	}
	evidenceA := evidenceFromJSON(target.EvidenceA)
	evidenceB := evidenceFromJSON(target.EvidenceB)
	chosen, other := evidenceA, evidenceB
	sideLabel := "A"
	if choice == "b" {
		chosen, other = evidenceB, evidenceA
		sideLabel = "B"
	}
	subject := readingTruncateRunes(strings.TrimSpace(target.Description), 120)
	resolutionText := note
	if choice == "a" || choice == "b" {
		if resolutionText == "" {
			resolutionText = fmt.Sprintf("关于「%s」：以 %s 方为准", subject, sideLabel)
		} else {
			resolutionText = fmt.Sprintf("关于「%s」：以 %s 方为准。%s", subject, sideLabel, resolutionText)
		}
	} else if resolutionText == "" {
		resolutionText = fmt.Sprintf("关于「%s」：用户已裁定", subject)
	} else {
		resolutionText = fmt.Sprintf("关于「%s」：%s", subject, resolutionText)
	}
	decisionEvidence := chosen
	if choice == "custom" {
		decisionEvidence = append(append([]types.ReadingEvidence{}, evidenceA...), evidenceB...)
	}
	now := time.Now()
	target.Status = types.ReadingContradictionResolved
	target.Resolution = resolutionText
	target.UpdatedAt = now
	gates := types.ReadingConclusionGates{
		ReferencesOK: len(decisionEvidence) > 0, ConflictFree: true,
		CrossDocConsistent: true, UserAdjudicated: true,
	}
	rawGates, _ := json.Marshal(gates)
	conclusion := &types.ReadingConclusion{
		ID: uuid.NewString(), RunID: run.ID, TenantID: run.TenantID,
		Round: run.StepCount, Text: resolutionText,
		Evidence: evidenceJSON(decisionEvidence), Status: types.ReadingConclusionConfirmed,
		Source: "user", Gates: types.JSON(rawGates), CreatedAt: now, UpdatedAt: now,
	}

	updated := make([]*types.ReadingConclusion, 0)
	if choice == "a" || choice == "b" {
		conclusions, err := s.repo.ListConclusions(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		chosenChunks := map[string]struct{}{}
		otherChunks := map[string]struct{}{}
		for _, evidence := range chosen {
			chosenChunks[evidence.ChunkID] = struct{}{}
		}
		for _, evidence := range other {
			otherChunks[evidence.ChunkID] = struct{}{}
		}
		for _, item := range conclusions {
			if item.Status != types.ReadingConclusionContradicted {
				continue
			}
			touchesChosen := false
			touchesOther := false
			for _, evidence := range evidenceFromJSON(item.Evidence) {
				if _, ok := chosenChunks[evidence.ChunkID]; ok {
					touchesChosen = true
				}
				if _, ok := otherChunks[evidence.ChunkID]; ok {
					touchesOther = true
				}
			}
			if !touchesChosen || touchesOther {
				continue
			}
			item.Status = types.ReadingConclusionConfirmed
			itemGates := types.ReadingConclusionGates{
				ReferencesOK: true, ConflictFree: true,
				CrossDocConsistent: true, UserAdjudicated: true,
			}
			raw, _ := json.Marshal(itemGates)
			item.Gates = types.JSON(raw)
			item.UpdatedAt = now
			updated = append(updated, item)
		}
	}
	if err := s.repo.ResolveContradiction(ctx, target, conclusion, updated); err != nil {
		return nil, err
	}
	// The structured state changed after a possible report edit; a stale draft
	// must not silently override the new decision in Markdown export.
	_, _ = s.repo.DeleteReportDraft(ctx, run.TenantID, run.UserID, run.ID, -1)
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) GetReport(ctx context.Context, runID string) (*types.ReadingReport, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if plan.StateVersion < readingStructuredStateVersion {
		return nil, werrors.NewValidationError("仅结构化精读支持报告编辑")
	}
	if err := s.checkReadingDocumentsReadable(ctx, caller, run); err != nil {
		return nil, err
	}
	draft, err := s.repo.GetReportDraft(ctx, caller.TenantID, caller.UserID, run.ID)
	if err != nil {
		return nil, err
	}
	if draft != nil && strings.TrimSpace(draft.ContentMD) != "" {
		updated := draft.UpdatedAt
		return &types.ReadingReport{
			Content: draft.ContentMD, Edited: true,
			Version: draft.Version, UpdatedAt: &updated,
		}, nil
	}
	content, _, _ := s.structuredReportContent(run, &plan)
	return &types.ReadingReport{Content: content, Edited: false, Version: 0}, nil
}

func (s *readingRunService) SaveReport(
	ctx context.Context, runID string, req *types.SaveReadingReportRequest,
) (*types.ReadingReport, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if plan.StateVersion < readingStructuredStateVersion {
		return nil, werrors.NewValidationError("仅结构化精读支持报告编辑")
	}
	if run.Status != types.ReadingRunFinished && run.Status != types.ReadingRunStopped {
		return nil, werrors.NewValidationError("精读结束后才能编辑报告")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, werrors.NewValidationError("报告内容不能为空")
	}
	if len([]rune(content)) > 200000 {
		return nil, werrors.NewValidationError("报告长度超出限制")
	}
	if err := s.checkReadingDocumentsReadable(ctx, caller, run); err != nil {
		return nil, err
	}
	draft := &types.ReadingReportDraft{
		RunID: run.ID, TenantID: caller.TenantID, UserID: caller.UserID,
		ContentMD: content, UpdatedAt: time.Now(),
	}
	baseVersion := req.BaseVersion
	if baseVersion < 0 {
		baseVersion = 0
	}
	saved, err := s.repo.SaveReportDraft(ctx, draft, baseVersion)
	if err != nil {
		return nil, err
	}
	if !saved {
		return nil, werrors.NewConflictError("报告已被其他人修改，请重新加载后再保存")
	}
	updated := draft.UpdatedAt
	return &types.ReadingReport{
		Content: content, Edited: true, Version: draft.Version, UpdatedAt: &updated,
	}, nil
}

func (s *readingRunService) DeleteReport(
	ctx context.Context, runID string, baseVersion int,
) (*types.ReadingReport, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if plan.StateVersion < readingStructuredStateVersion {
		return nil, werrors.NewValidationError("仅结构化精读支持报告编辑")
	}
	if err := s.checkReadingDocumentsReadable(ctx, caller, run); err != nil {
		return nil, err
	}
	deleted, err := s.repo.DeleteReportDraft(
		ctx, caller.TenantID, caller.UserID, run.ID, baseVersion,
	)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, werrors.NewConflictError("报告已被其他人修改，请重新加载后再重置")
	}
	content, _, _ := s.structuredReportContent(run, &plan)
	return &types.ReadingReport{Content: content, Edited: false, Version: 0}, nil
}

// GetDocuments returns the selected documents as full text. Chunks are joined
// in reading order without chunk markers; the left pane shows one complete
// document at a time. Every document is re-checked against ACL before its text
// leaves the server.
func (s *readingRunService) GetDocuments(
	ctx context.Context, runID string,
) (*types.ReadingRunDocuments, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	var documentIDs []string
	_ = json.Unmarshal(run.DocumentIDs, &documentIDs)
	planSectionsByDoc := map[string][]types.ReadingSection{}
	for _, section := range plan.Sections {
		planSectionsByDoc[section.KnowledgeID] = append(
			planSectionsByDoc[section.KnowledgeID], section,
		)
	}
	documents := make([]types.ReadingDocument, 0, len(documentIDs))
	for _, docID := range documentIDs {
		knowledge, err := s.ensureDocumentReadable(ctx, caller, docID)
		if err != nil {
			return nil, err
		}
		chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, caller.TenantID, knowledge.ID)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(chunks, func(i, j int) bool {
			return chunks[i].ChunkIndex < chunks[j].ChunkIndex
		})
		var builder strings.Builder
		chunkStart := map[string]int{}
		chunkLength := map[string]int{}
		contentLength := 0
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			content := strings.TrimSpace(chunk.Content)
			if content == "" {
				continue
			}
			if contentLength > 0 {
				builder.WriteString("\n\n")
				contentLength += 2
			}
			chunkStart[chunk.ID] = contentLength
			builder.WriteString(content)
			length := utf16Length(content)
			chunkLength[chunk.ID] = length
			contentLength += length
		}
		title := knowledge.Title
		if strings.TrimSpace(title) == "" {
			title = knowledge.FileName
		}
		sections := make([]types.ReadingDocumentSection, 0, len(planSectionsByDoc[knowledge.ID]))
		for _, section := range planSectionsByDoc[knowledge.ID] {
			start, end := 0, 0
			if len(section.ChunkIDs) > 0 {
				first, last := section.ChunkIDs[0], section.ChunkIDs[len(section.ChunkIDs)-1]
				if firstStart, ok := chunkStart[first]; ok {
					start = firstStart
					if lastStart, ok := chunkStart[last]; ok {
						end = lastStart + chunkLength[last]
					}
				}
			}
			sections = append(sections, types.ReadingDocumentSection{
				ID: section.ID, Title: section.Title, Heading: section.Heading,
				Read: section.Read, Start: start, End: end,
			})
		}
		documents = append(documents, types.ReadingDocument{
			ID: knowledge.ID, Title: title, FileName: knowledge.FileName,
			Content: builder.String(), Sections: sections,
		})
	}
	return &types.ReadingRunDocuments{Documents: documents}, nil
}

func utf16Length(value string) int {
	length := 0
	for _, r := range value {
		if n := utf16.RuneLen(r); n > 0 {
			length += n
		}
	}
	return length
}

func (s *readingRunService) checkReadingDocumentsReadable(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
) error {
	for _, docID := range planDocumentIDs(run) {
		if _, err := s.ensureDocumentReadable(ctx, caller, docID); err != nil {
			return werrors.NewForbiddenError("文档权限已变更，无法查看或导出报告")
		}
	}
	return nil
}

// structuredReportContent projects the four state tables into the editable
// Markdown report. The same projection is used by the editor and by export so
// the two never drift.
func (s *readingRunService) structuredReportContent(
	run *types.ReadingRun, plan *types.ReadingPlan,
) (string, types.ReadingCoverage, []string) {
	documentIDs := planDocumentIDs(run)
	docTitles := make([]string, 0)
	seenTitles := map[string]struct{}{}
	readSections := 0
	for _, section := range plan.Sections {
		if _, ok := seenTitles[section.KnowledgeID]; !ok {
			seenTitles[section.KnowledgeID] = struct{}{}
			title := section.Title
			if strings.TrimSpace(title) == "" {
				title = section.KnowledgeID
			}
			docTitles = append(docTitles, title)
		}
		if section.Read {
			readSections++
		}
	}
	coverage := types.ReadingCoverage{
		Documents: len(documentIDs), TotalSections: len(plan.Sections),
		ReadSections: readSections, StepCount: run.StepCount, TokenUsage: run.TokenUsage,
	}
	final := map[string]any{}
	_ = json.Unmarshal(run.Final, &final)
	content := buildStructuredReadingMarkdown(
		run, *plan, run.Questions, run.Conclusions, run.Terms, run.Contradictions,
		final, docTitles, coverage, time.Now(),
	)
	return content, coverage, docTitles
}
