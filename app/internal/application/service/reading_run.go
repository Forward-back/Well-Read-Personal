package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"gaze/internal/application/repository"
	"gaze/internal/engine"
	werrors "gaze/internal/errors"
	"gaze/internal/logger"
	"gaze/internal/models/chat"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

const (
	readingMaxDocuments    = 5
	readingMaxSectionRunes = 8000
	readingMaxSteps        = 40
	readingMaxTokenBudget  = 50000
)

func normalizeReadingDepth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case types.ReadingDepthQuick:
		return types.ReadingDepthQuick
	case types.ReadingDepthDeep:
		return types.ReadingDepthDeep
	default:
		return types.ReadingDepthStandard
	}
}

func readingRunDepth(run *types.ReadingRun) string {
	if run == nil {
		return types.ReadingDepthStandard
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return types.ReadingDepthStandard
	}
	return normalizeReadingDepth(plan.Depth)
}

func readingDepthLabel(depth string) string {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return "快速通读"
	case types.ReadingDepthDeep:
		return "逐条精读"
	default:
		return "标准"
	}
}

func readingDepthGuidance(depth string) string {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return "阅读深度：快速通读。优先抓结构、结论与风险；只在关键歧义处提问，避免逐条追问。"
	case types.ReadingDepthDeep:
		return "阅读深度：逐条精读。逐项核对术语、数值、条件、例外与责任边界；发现疑点必须引用两处出处再提问。"
	default:
		return "阅读深度：标准。按章节或逻辑单元推进；在关键概念、矛盾、阶段结论处暂停确认。"
	}
}

func readingStepLimit(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 24
	case types.ReadingDepthDeep:
		return 60
	default:
		return readingMaxSteps
	}
}

func readingMaxTokens(depth string) int {
	switch normalizeReadingDepth(depth) {
	case types.ReadingDepthQuick:
		return 1500
	case types.ReadingDepthDeep:
		return 2500
	default:
		return 2000
	}
}

type readingRunService struct {
	repo          interfaces.ReadingRunRepository
	sessionRepo   interfaces.SessionRepository
	knowledgeRepo interfaces.KnowledgeRepository
	chunkRepo     interfaces.ChunkRepository
	kbService     interfaces.KnowledgeBaseService
	tenantService interfaces.TenantService
	modelService  interfaces.ModelService
	skillRepo     repository.TenantSkillRepository
	messageRepo   interfaces.MessageRepository
	ragEngine     engine.RAGEngine
}

func NewReadingRunService(
	repo interfaces.ReadingRunRepository,
	sessionRepo interfaces.SessionRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	kbService interfaces.KnowledgeBaseService,
	tenantService interfaces.TenantService,
	modelService interfaces.ModelService,
	skillRepo repository.TenantSkillRepository,
	messageRepo interfaces.MessageRepository,
	ragEngine engine.RAGEngine,
) interfaces.ReadingRunService {
	return &readingRunService{
		repo: repo, sessionRepo: sessionRepo, knowledgeRepo: knowledgeRepo,
		chunkRepo: chunkRepo, kbService: kbService,
		tenantService: tenantService, modelService: modelService,
		skillRepo: skillRepo, messageRepo: messageRepo,
		ragEngine: ragEngine,
	}
}

func readingCaller(ctx context.Context) (types.Caller, error) {
	caller := types.CallerFromContext(ctx).Normalize()
	if caller.TenantID == 0 || caller.UserID == "" {
		return caller, werrors.NewForbiddenError("需要登录后操作")
	}
	return caller, nil
}

func (s *readingRunService) Create(
	ctx context.Context, req *types.CreateReadingRunRequest,
) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	if s.sessionRepo == nil {
		return nil, werrors.NewInternalServerError("会话服务未配置")
	}
	session, err := s.sessionRepo.Get(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(req.SessionID))
	if err != nil || session == nil {
		return nil, werrors.NewNotFoundError("会话不存在")
	}
	if len(req.DocumentIDs) == 0 || len(req.DocumentIDs) > readingMaxDocuments {
		return nil, werrors.NewValidationError("精读文档数量需为 1-5 篇")
	}
	tenant, err := s.tenantService.GetTenantByID(ctx, caller.TenantID)
	if err != nil || tenant == nil {
		return nil, werrors.NewInternalServerError("读取空间配置失败")
	}
	if strings.TrimSpace(tenant.ReadingModelID) == "" {
		return nil, werrors.NewValidationError("空间未配置精读模型")
	}
	if err := s.ensureReadingModelSupportsThinking(ctx, tenant.ReadingModelID); err != nil {
		return nil, err
	}
	depth := normalizeReadingDepth(req.Depth)
	sections, docIDs, kbID, err := s.buildSections(ctx, caller, req.DocumentIDs, depth)
	if err != nil {
		return nil, err
	}
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		goal = defaultReadingGoal
	}
	plan := types.ReadingPlan{
		Goal:            goal,
		Depth:           depth,
		Policy:          types.ReadingPolicyGrillWithDocs,
		KnowledgeBaseID: kbID,
		StateVersion:    readingStructuredStateVersion,
		Sections:        sections,
	}
	planJSON, _ := json.Marshal(plan)
	notesJSON, _ := json.Marshal(types.ReadingNotes{})
	finalJSON, _ := json.Marshal(map[string]any{})
	docJSON, _ := json.Marshal(docIDs)
	now := time.Now()
	run := &types.ReadingRun{
		ID: uuid.NewString(), TenantID: caller.TenantID, SessionID: session.ID,
		UserID: caller.UserID, MessageID: strings.TrimSpace(req.MessageID),
		DocumentIDs: types.JSON(docJSON), SkillID: strings.TrimSpace(req.SkillID),
		ReadingModelID: strings.TrimSpace(tenant.ReadingModelID),
		Status:         types.ReadingRunPlanning, Plan: types.JSON(planJSON),
		Notes: types.JSON(notesJSON), Final: types.JSON(finalJSON),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.StopActiveRuns(ctx, caller.TenantID, caller.UserID, session.ID); err != nil {
		return nil, err
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	if s.sessionRepo != nil {
		if _, err := s.sessionRepo.SetReadingRunID(ctx, caller.TenantID, caller.UserID, session.ID, run.ID); err != nil {
			logger.Warnf(ctx, "reading: link session %s to run %s failed: %v", session.ID, run.ID, err)
		}
	}
	if s.messageRepo != nil && run.MessageID != "" {
		if message, msgErr := s.messageRepo.GetMessage(ctx, session.ID, run.MessageID); msgErr == nil && message != nil {
			message.ReadingRunID = run.ID
			if msgErr = s.messageRepo.UpdateMessage(ctx, message); msgErr != nil {
				logger.Warnf(ctx, "reading: link message %s failed: %v", run.MessageID, msgErr)
			}
		}
	}
	stripReadingPlanContent(run)
	return run, nil
}

func (s *readingRunService) Get(ctx context.Context, runID string) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	steps, err := s.repo.ListSteps(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	run.Steps = steps
	if err := s.maybeMigrateLegacyRun(ctx, run); err != nil {
		logger.Warnf(ctx, "reading: migrate legacy run %s failed: %v", run.ID, err)
	}
	if err := s.populateStructuredReading(ctx, run); err != nil {
		return nil, err
	}
	stripReadingPlanContent(run)
	return run, nil
}

func (s *readingRunService) GetActive(ctx context.Context, sessionID string) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetActiveRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, werrors.NewNotFoundError("没有进行中的精读任务")
	}
	steps, err := s.repo.ListSteps(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	run.Steps = steps
	if err := s.maybeMigrateLegacyRun(ctx, run); err != nil {
		logger.Warnf(ctx, "reading: migrate legacy run %s failed: %v", run.ID, err)
	}
	if err := s.populateStructuredReading(ctx, run); err != nil {
		return nil, err
	}
	stripReadingPlanContent(run)
	return run, nil
}

func (s *readingRunService) Step(ctx context.Context, runID string) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	switch run.Status {
	case types.ReadingRunFinished, types.ReadingRunStopped:
		return nil, werrors.NewValidationError("精读任务已结束")
	case types.ReadingRunWaitingUser:
		return nil, werrors.NewValidationError("请先反馈当前问题")
	}
	var plan types.ReadingPlan
	_ = json.Unmarshal(run.Plan, &plan)
	if plan.StateVersion >= readingStructuredStateVersion {
		return s.structuredStep(ctx, caller, run)
	}
	return s.legacyStep(ctx, caller, run)
}

func (s *readingRunService) legacyStep(
	ctx context.Context, caller types.Caller, run *types.ReadingRun,
) (*types.ReadingRun, error) {
	if run.StepCount >= readingStepLimit(readingRunDepth(run)) {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, werrors.NewValidationError("已达到最大步数，精读已停止")
	}
	if run.TokenUsage >= readingMaxTokenBudget {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, werrors.NewValidationError("已达到 token 预算，精读已停止")
	}
	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	var notes types.ReadingNotes
	_ = json.Unmarshal(run.Notes, &notes)

	sectionIndex := -1
	for i := range plan.Sections {
		if !plan.Sections[i].Read {
			sectionIndex = i
			break
		}
	}
	run.Status = types.ReadingRunReading
	if sectionIndex == -1 {
		final, usage, err := s.finishWithLLM(ctx, run, notes)
		if err != nil {
			return nil, err
		}
		finalJSON, _ := json.Marshal(final)
		run.Final = types.JSON(finalJSON)
		run.Status = types.ReadingRunFinished
		run.TokenUsage += usage
		run.StepCount++
		run.UpdatedAt = time.Now()
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
		step := &types.ReadingRunStep{
			ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
			Action: "finish", Question: types.JSON("{}"),
			NotesPatch: types.JSON("{}"), Citations: types.JSON("[]"),
			Status: "done", CreatedAt: time.Now(),
		}
		_ = s.repo.CreateStep(ctx, step)
		return s.Get(ctx, run.ID)
	}

	section := &plan.Sections[sectionIndex]
	// Re-check KB/ACL access on every step: a deny added after the run was
	// created must stop the read before its content reaches the model.
	if _, err := s.ensureDocumentReadable(ctx, caller, section.KnowledgeID); err != nil {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, err
	}
	resp, usage, err := s.stepWithLLM(ctx, caller, run, section, notes)
	if err != nil {
		run.Status = types.ReadingRunFailed
		run.UpdatedAt = time.Now()
		_ = s.repo.UpdateRun(ctx, run)
		return nil, err
	}
	section.Read = true
	planJSON, _ := json.Marshal(plan)
	run.Plan = types.JSON(planJSON)
	run.CurrentSectionID = section.ID
	mergeReadingNotes(&notes, resp.NotesPatch)
	notesJSON, _ := json.Marshal(notes)
	run.Notes = types.JSON(notesJSON)
	run.StepCount++
	run.TokenUsage += usage
	run.UpdatedAt = time.Now()

	action := "update_notes"
	questionJSON, _ := json.Marshal(types.ReadingQuestion{})
	patchJSON, _ := json.Marshal(resp.NotesPatch)
	citationsJSON, _ := json.Marshal(resp.NotesPatch.Citations)
	if resp.Question != nil && strings.TrimSpace(resp.Question.Text) != "" {
		action = "ask_user"
		run.Status = types.ReadingRunWaitingUser
		questionJSON, _ = json.Marshal(resp.Question)
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	step := &types.ReadingRunStep{
		ID: uuid.NewString(), RunID: run.ID, Seq: run.StepCount,
		Action: action, SectionID: section.ID,
		Question: types.JSON(questionJSON), NotesPatch: types.JSON(patchJSON),
		Citations: types.JSON(citationsJSON), Status: "done", CreatedAt: time.Now(),
	}
	if err := s.repo.CreateStep(ctx, step); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) Feedback(
	ctx context.Context, runID string, req *types.ReadingFeedbackRequest,
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
	_ = json.Unmarshal(run.Plan, &plan)
	if plan.StateVersion >= readingStructuredStateVersion {
		return s.structuredFeedback(ctx, caller, run, req)
	}
	return s.legacyFeedback(ctx, caller, run, req)
}

func (s *readingRunService) legacyFeedback(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, req *types.ReadingFeedbackRequest,
) (*types.ReadingRun, error) {
	feedback := strings.TrimSpace(req.Feedback)
	if feedback == "" {
		return nil, werrors.NewValidationError("反馈不能为空")
	}
	if run.Status != types.ReadingRunWaitingUser {
		return nil, werrors.NewValidationError("当前没有待反馈的问题")
	}
	steps, err := s.repo.ListSteps(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	var pending *types.ReadingRunStep
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Action == "ask_user" && steps[i].UserFeedback == "" {
			pending = steps[i]
			break
		}
	}
	if pending == nil {
		return nil, werrors.NewValidationError("待反馈问题不存在")
	}
	pending.UserFeedback = feedback
	pending.Status = "answered"
	if err := s.repo.UpdateStep(ctx, pending); err != nil {
		return nil, err
	}
	var notes types.ReadingNotes
	_ = json.Unmarshal(run.Notes, &notes)
	notes.Confirmed = readingAppendUnique(notes.Confirmed, "用户反馈："+feedback)
	notesJSON, _ := json.Marshal(notes)
	run.Notes = types.JSON(notesJSON)
	run.Status = types.ReadingRunReading
	run.UpdatedAt = time.Now()
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) AddDocuments(
	ctx context.Context, runID string, req *types.AddReadingDocumentsRequest,
) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil || run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	if run.Status == types.ReadingRunFinished || run.Status == types.ReadingRunStopped {
		return nil, werrors.NewValidationError("精读任务已结束")
	}
	var existing []string
	_ = json.Unmarshal(run.DocumentIDs, &existing)
	if len(existing)+len(req.DocumentIDs) > readingMaxDocuments {
		return nil, werrors.NewValidationError("精读文档最多 5 篇")
	}
	newSections, newDocs, newKBID, err := s.buildSections(ctx, caller, req.DocumentIDs, readingRunDepth(run))
	if err != nil {
		return nil, err
	}
	var plan types.ReadingPlan
	_ = json.Unmarshal(run.Plan, &plan)
	if plan.KnowledgeBaseID != "" && newKBID != "" && plan.KnowledgeBaseID != newKBID {
		return nil, werrors.NewValidationError("一次精读只能同一知识库")
	}
	if plan.KnowledgeBaseID == "" {
		plan.KnowledgeBaseID = newKBID
	}
	plan.Sections = append(plan.Sections, newSections...)
	planJSON, _ := json.Marshal(plan)
	existing = append(existing, newDocs...)
	docJSON, _ := json.Marshal(readingUniqueStrings(existing))
	run.Plan = types.JSON(planJSON)
	run.DocumentIDs = types.JSON(docJSON)
	run.Status = types.ReadingRunReading
	if plan.StateVersion >= readingStructuredStateVersion {
		if pending := s.countPendingReadingQuestions(ctx, run.ID); pending > 0 {
			run.Status = types.ReadingRunWaitingUser
		}
	}
	run.UpdatedAt = time.Now()
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) Stop(ctx context.Context, runID string) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil || run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	if run.Status != types.ReadingRunFinished && run.Status != types.ReadingRunStopped {
		run.Status = types.ReadingRunStopped
		run.UpdatedAt = time.Now()
		if err := s.repo.UpdateRun(ctx, run); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) Resume(ctx context.Context, runID string) (*types.ReadingRun, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetRun(ctx, caller.TenantID, caller.UserID, strings.TrimSpace(runID))
	if err != nil || run == nil {
		return nil, werrors.NewNotFoundError("精读任务不存在")
	}
	if run.Status != types.ReadingRunStopped && run.Status != types.ReadingRunFailed {
		return nil, werrors.NewValidationError("仅已停止或失败的精读可以继续")
	}
	var plan types.ReadingPlan
	_ = json.Unmarshal(run.Plan, &plan)
	if plan.StateVersion >= readingStructuredStateVersion {
		depth := readingRunDepth(run)
		if run.StepCount >= readingStructuredStepLimit(depth) {
			return nil, werrors.NewValidationError("已达到最大步数，不能继续")
		}
		if run.TokenUsage >= readingStructuredTokenBudget(depth) {
			return nil, werrors.NewValidationError("已达到 token 预算，不能继续")
		}
		if pending := s.countPendingReadingQuestions(ctx, run.ID); pending > 0 {
			run.Status = types.ReadingRunWaitingUser
		} else {
			run.Status = types.ReadingRunReading
		}
		// Resuming a stopped run can add findings, so an edited report from the
		// previous stop is stale and must not silently override the new one.
		if _, err := s.repo.DeleteReportDraft(ctx, run.TenantID, run.UserID, run.ID, -1); err != nil {
			logger.Warnf(ctx, "reading: delete report draft %s failed: %v", run.ID, err)
		}
	} else {
		if run.StepCount >= readingStepLimit(readingRunDepth(run)) {
			return nil, werrors.NewValidationError("已达到最大步数，不能继续")
		}
		if run.TokenUsage >= readingMaxTokenBudget {
			return nil, werrors.NewValidationError("已达到 token 预算，不能继续")
		}
		run.Status = types.ReadingRunReading
	}
	run.UpdatedAt = time.Now()
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	return s.Get(ctx, run.ID)
}

func (s *readingRunService) Export(ctx context.Context, runID string, format string) (*types.ReadingExport, error) {
	caller, err := readingCaller(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.Get(ctx, runID)
	if err != nil {
		return nil, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "markdown"
	}
	if format != "markdown" && format != "json" {
		return nil, werrors.NewValidationError("导出格式仅支持 markdown 或 json")
	}

	var plan types.ReadingPlan
	if err := json.Unmarshal(run.Plan, &plan); err != nil {
		return nil, werrors.NewInternalServerError("阅读计划损坏")
	}
	if plan.StateVersion >= readingStructuredStateVersion {
		return s.exportStructuredReading(ctx, caller, run, &plan, format)
	}
	var notes types.ReadingNotes
	_ = json.Unmarshal(run.Notes, &notes)
	final := map[string]any{}
	_ = json.Unmarshal(run.Final, &final)

	docTitles := make([]string, 0)
	seenTitles := map[string]struct{}{}
	for _, section := range plan.Sections {
		if _, ok := seenTitles[section.KnowledgeID]; ok {
			continue
		}
		seenTitles[section.KnowledgeID] = struct{}{}
		title := section.Title
		if strings.TrimSpace(title) == "" {
			title = section.KnowledgeID
		}
		docTitles = append(docTitles, title)
	}
	readSections := 0
	for _, section := range plan.Sections {
		if section.Read {
			readSections++
		}
	}

	var documentIDs []string
	_ = json.Unmarshal(run.DocumentIDs, &documentIDs)
	for _, docID := range documentIDs {
		if _, err := s.ensureDocumentReadable(ctx, caller, docID); err != nil {
			return nil, werrors.NewForbiddenError("文档权限已变更，无法导出；请移除无权文档后重试")
		}
	}

	coverage := types.ReadingCoverage{
		Documents:     len(documentIDs),
		TotalSections: len(plan.Sections),
		ReadSections:  readSections,
		StepCount:     run.StepCount,
		TokenUsage:    run.TokenUsage,
	}
	generatedAt := time.Now()
	result := &types.ReadingExport{
		Format:      format,
		Coverage:    coverage,
		GeneratedAt: generatedAt,
	}
	if format == "json" {
		payload := map[string]any{
			"run_id":       run.ID,
			"goal":         plan.Goal,
			"depth":        plan.Depth,
			"policy":       plan.Policy,
			"documents":    docTitles,
			"coverage":     coverage,
			"notes":        notes,
			"final":        final,
			"generated_at": generatedAt,
		}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, err
		}
		result.Content = string(raw)
		result.Filename = fmt.Sprintf("wellread-reading-%s.json", run.ID)
	} else {
		result.Content = buildReadingMarkdown(run, plan, notes, final, docTitles, coverage, generatedAt)
		result.Filename = fmt.Sprintf("wellread-reading-%s.md", run.ID)
	}
	return result, nil
}

func buildReadingMarkdown(
	run *types.ReadingRun, plan types.ReadingPlan, notes types.ReadingNotes,
	final map[string]any, docTitles []string, coverage types.ReadingCoverage, generatedAt time.Time,
) string {
	var b strings.Builder
	writeList := func(values []string) {
		if len(values) == 0 {
			b.WriteString("—\n")
			return
		}
		for _, value := range values {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(value))
			b.WriteString("\n")
		}
	}
	writeAnyList := func(values any) {
		list, _ := values.([]any)
		if len(list) == 0 {
			b.WriteString("—\n")
			return
		}
		for _, value := range list {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(fmt.Sprint(value)))
			b.WriteString("\n")
		}
	}

	b.WriteString("# WellRead 精读报告\n\n")
	b.WriteString("- 阅读目标：" + strings.TrimSpace(plan.Goal) + "\n")
	b.WriteString("- 阅读深度：" + readingDepthLabel(plan.Depth) + "\n")
	b.WriteString("- 精读策略：" + plan.Policy + "\n")
	b.WriteString("- 文档数量：" + fmt.Sprint(coverage.Documents) + "\n")
	b.WriteString("- 覆盖：" + fmt.Sprint(coverage.ReadSections) + "/" + fmt.Sprint(coverage.TotalSections) + " 个阅读单元\n")
	b.WriteString("- 生成时间：" + generatedAt.Format(time.RFC3339) + "\n\n")
	b.WriteString("## 文档\n\n")
	writeList(docTitles)
	b.WriteString("\n## 结论摘要\n\n")
	summary, _ := final["summary"].(string)
	if summary = strings.TrimSpace(summary); summary != "" {
		b.WriteString(summary + "\n")
	} else {
		b.WriteString("—\n")
	}
	b.WriteString("\n### 关键结论\n\n")
	writeAnyList(final["key_points"])
	b.WriteString("\n### 未决问题\n\n")
	writeAnyList(final["open_questions"])
	b.WriteString("\n### 矛盾\n\n")
	writeAnyList(final["contradictions"])
	b.WriteString("\n## 阅读笔记\n\n")
	for _, group := range []struct {
		label  string
		values []string
	}{
		{"已确认", notes.Confirmed},
		{"术语", notes.Terms},
		{"未决问题", notes.OpenQuestions},
		{"矛盾", notes.Contradictions},
		{"用户裁定", notes.Decisions},
		{"引用", notes.Citations},
	} {
		b.WriteString("### " + group.label + "\n\n")
		writeList(group.values)
		b.WriteString("\n")
	}
	b.WriteString("## 覆盖与局限\n\n")
	b.WriteString("- 仅基于用户选定的文档，不引入集合外资料。\n")
	b.WriteString("- 已读 " + fmt.Sprint(coverage.ReadSections) + "/" + fmt.Sprint(coverage.TotalSections) + " 个阅读单元；未覆盖部分未纳入结论。\n")
	b.WriteString("- 报告为阅读结论，不是公司正式文件；对外使用前需人工确认。\n")
	b.WriteString("\n> run_id: " + run.ID + "\n")
	return b.String()
}

func (s *readingRunService) ensureDocumentReadable(
	ctx context.Context, caller types.Caller, docID string,
) (*types.Knowledge, error) {
	knowledge, err := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, docID)
	if err != nil || knowledge == nil || knowledge.TenantID != caller.TenantID {
		return nil, werrors.NewNotFoundError("文档不存在")
	}
	if knowledge.ParseStatus != types.ParseStatusCompleted {
		return nil, werrors.NewValidationError("文档尚未解析完成，不能精读")
	}
	fileType := strings.ToLower(strings.TrimSpace(knowledge.FileType))
	if fileType == "xlsx" || fileType == "xls" || strings.HasPrefix(fileType, "image/") ||
		strings.HasPrefix(fileType, "audio/") || strings.HasPrefix(fileType, "video/") {
		return nil, werrors.NewValidationError("v1 不支持该类型文档全文精读")
	}
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
	if err != nil || kb == nil {
		return nil, werrors.NewNotFoundError("知识库不存在")
	}
	readable, err := s.canReadKB(ctx, caller, kb)
	if err != nil {
		return nil, err
	}
	if !readable {
		return nil, werrors.NewForbiddenError("无权精读该文档")
	}
	return knowledge, nil
}

func (s *readingRunService) buildSections(
	ctx context.Context, caller types.Caller, documentIDs []string, depth string,
) ([]types.ReadingSection, []string, string, error) {
	sections := make([]types.ReadingSection, 0)
	docIDs := make([]string, 0, len(documentIDs))
	kbID := ""
	seen := make(map[string]struct{}, len(documentIDs))
	for _, docID := range documentIDs {
		docID = strings.TrimSpace(docID)
		if docID == "" {
			continue
		}
		if _, ok := seen[docID]; ok {
			continue
		}
		seen[docID] = struct{}{}
		knowledge, err := s.ensureDocumentReadable(ctx, caller, docID)
		if err != nil {
			return nil, nil, "", err
		}
		if kbID == "" {
			kbID = knowledge.KnowledgeBaseID
		} else if knowledge.KnowledgeBaseID != kbID {
			return nil, nil, "", werrors.NewValidationError("一次精读只能同一知识库")
		}
		chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, caller.TenantID, knowledge.ID)
		if err != nil {
			return nil, nil, "", err
		}
		sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].ChunkIndex < chunks[j].ChunkIndex })
		docSections := buildDocSections(knowledge, chunks, readingWindowRunes(depth))
		if len(docSections) == 0 {
			return nil, nil, "", werrors.NewValidationError("文档没有可读文本")
		}
		sections = append(sections, docSections...)
		docIDs = append(docIDs, knowledge.ID)
	}
	if len(sections) == 0 {
		return nil, nil, "", werrors.NewValidationError("没有可精读的文档")
	}
	return sections, docIDs, kbID, nil
}

func (s *readingRunService) canReadKB(
	ctx context.Context, caller types.Caller, kb *types.KnowledgeBase,
) (bool, error) {
	if kb == nil {
		return false, nil
	}
	return kb.TenantID == caller.TenantID, nil
}

func buildDocSections(knowledge *types.Knowledge, chunks []*types.Chunk, windowRunes int) []types.ReadingSection {
	title := knowledge.Title
	if title == "" {
		title = knowledge.FileName
	}
	if windowRunes <= 0 {
		windowRunes = readingMaxSectionRunes
	}
	sections := make([]types.ReadingSection, 0)
	var currentHeader string
	var current []*types.Chunk
	flush := func() {
		if len(current) == 0 {
			return
		}
		segments := make([]types.ReadingChunkSegment, 0, len(current))
		for _, chunk := range current {
			if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
				continue
			}
			heading := strings.TrimSpace(chunk.ContextHeader)
			for _, part := range splitRunes(strings.TrimSpace(chunk.Content), windowRunes) {
				segments = append(segments, types.ReadingChunkSegment{
					ID: chunk.ID, Heading: heading, Content: part,
				})
			}
		}
		var window []types.ReadingChunkSegment
		size := 0
		flushWindow := func() {
			if len(window) == 0 {
				return
			}
			parts := make([]string, 0, len(window))
			chunkIDs := make([]string, 0, len(window))
			seenChunks := map[string]struct{}{}
			for _, segment := range window {
				parts = append(parts, segment.Content)
				if _, ok := seenChunks[segment.ID]; !ok {
					seenChunks[segment.ID] = struct{}{}
					chunkIDs = append(chunkIDs, segment.ID)
				}
			}
			idx := len(sections)
			sections = append(sections, types.ReadingSection{
				ID:          fmt.Sprintf("%s-%d", knowledge.ID, idx),
				KnowledgeID: knowledge.ID,
				Title:       title,
				Heading:     window[0].Heading,
				Index:       idx,
				Content:     strings.Join(parts, "\n\n"),
				ChunkIDs:    chunkIDs,
				Segments:    append([]types.ReadingChunkSegment(nil), window...),
			})
			window = nil
			size = 0
		}
		for _, segment := range segments {
			segmentLen := len([]rune(segment.Content))
			if size > 0 && size+segmentLen > windowRunes {
				flushWindow()
			}
			window = append(window, segment)
			size += segmentLen
			if size >= windowRunes {
				flushWindow()
			}
		}
		flushWindow()
		current = nil
	}
	for _, chunk := range chunks {
		if chunk == nil || strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		header := strings.TrimSpace(chunk.ContextHeader)
		if header != currentHeader && len(current) > 0 {
			flush()
		}
		currentHeader = header
		current = append(current, chunk)
	}
	flush()
	return sections
}

func splitRunes(value string, max int) []string {
	runes := []rune(value)
	if len(runes) <= max {
		return []string{value}
	}
	out := make([]string, 0, (len(runes)+max-1)/max)
	for len(runes) > 0 {
		n := max
		if len(runes) < n {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func (s *readingRunService) readingInstructions(ctx context.Context, run *types.ReadingRun) string {
	// 精读固定使用内置 grill-with-docs 策略；技能只用于普通问答。
	return readingGrillWithDocsPolicy + "\n\n" + readingDepthGuidance(readingRunDepth(run))
}

type readingLLMResponse struct {
	Action      string                  `json:"action"`
	SearchQuery string                  `json:"search_query"`
	NotesPatch  types.ReadingNotesPatch `json:"notes_patch"`
	Question    *types.ReadingQuestion  `json:"question"`
	Final       map[string]any          `json:"final"`
}

func (s *readingRunService) stepWithLLM(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, section *types.ReadingSection, notes types.ReadingNotes,
) (*readingLLMResponse, int, error) {
	model, err := s.modelService.GetChatModel(ctx, run.ReadingModelID)
	if err != nil {
		return nil, 0, fmt.Errorf("加载精读模型失败: %w", err)
	}
	notesJSON, _ := json.Marshal(notes)
	depth := readingRunDepth(run)
	userPrompt := fmt.Sprintf(
		"阅读目标：%s\n阅读深度：%s\n\n已有笔记：%s\n\n当前章节：%s § %s\n\n章节内容：\n%s\n\n"+
			"返回严格 JSON：{\"action\":\"update_notes|ask_user|search_selected_docs\",\"search_query\":\"\",\"notes_patch\":{\"confirmed\":[],\"terms\":[],\"open_questions\":[],\"contradictions\":[],\"decisions\":[],\"citations\":[]},\"question\":{\"text\":\"\",\"recommended_answer\":\"\",\"why\":\"\"}}。最多一个问题；能读文档回答的不问用户；需要跨章节定位时先 search_selected_docs，只允许检索已选文档。",
		readingGoal(run), readingDepthLabel(depth), string(notesJSON), section.Title, section.Heading, section.Content,
	)
	messages := []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run)},
		{Role: "user", Content: userPrompt},
	}
	totalUsage := 0
	for round := 0; round < 4; round++ {
		resp, err := model.Chat(ctx, messages, &chat.ChatOptions{Temperature: 0.2, MaxTokens: readingMaxTokens(depth)})
		if err != nil {
			return nil, totalUsage, fmt.Errorf("精读模型调用失败: %w", err)
		}
		totalUsage += resp.Usage.TotalTokens
		parsed, err := parseReadingLLM(resp.Content)
		if err != nil {
			return nil, totalUsage, err
		}
		if parsed.Action != "search_selected_docs" {
			return parsed, totalUsage, nil
		}
		results, err := s.searchSelectedDocs(ctx, caller, run, parsed.SearchQuery)
		if err != nil {
			return nil, totalUsage, err
		}
		messages = append(messages,
			chat.Message{Role: "assistant", Content: resp.Content},
			chat.Message{Role: "user", Content: "仅基于已选文档的检索结果：\n" + results + "\n\n请继续按严格 JSON 返回 update_notes 或 ask_user；不要引入检索结果之外的资料。"},
		)
	}
	return nil, totalUsage, fmt.Errorf("精读检索次数超限")
}

func (s *readingRunService) searchSelectedDocs(
	ctx context.Context, caller types.Caller, run *types.ReadingRun, query string,
) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", werrors.NewValidationError("精读检索缺少检索词")
	}
	var documentIDs []string
	_ = json.Unmarshal(run.DocumentIDs, &documentIDs)
	var b strings.Builder
	count := 0
	for _, docID := range documentIDs {
		knowledge, err := s.ensureDocumentReadable(ctx, caller, docID)
		if err != nil {
			return "", err
		}
		chunks, err := s.chunkRepo.ListChunksByKnowledgeID(ctx, caller.TenantID, knowledge.ID)
		if err != nil {
			return "", err
		}
		title := knowledge.Title
		if strings.TrimSpace(title) == "" {
			title = knowledge.FileName
		}
		for _, chunk := range chunks {
			if chunk == nil || !readingChunkMatches(chunk.Content, query) {
				continue
			}
			fmt.Fprintf(&b, "[%s · 块 %d] %s\n\n", title, chunk.ChunkIndex, readingTruncateRunes(chunk.Content, 600))
			count++
			if count >= 8 {
				break
			}
		}
		if count >= 8 {
			break
		}
	}
	if count == 0 {
		return fmt.Sprintf("在已选文档中未找到与「%s」匹配的内容。", query), nil
	}
	return b.String(), nil
}

func readingChunkMatches(content, query string) bool {
	if strings.Contains(content, query) {
		return true
	}
	for _, token := range strings.Fields(query) {
		if len([]rune(token)) >= 2 && strings.Contains(content, token) {
			return true
		}
	}
	return false
}

func readingTruncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

func (s *readingRunService) finishWithLLM(
	ctx context.Context, run *types.ReadingRun, notes types.ReadingNotes,
) (map[string]any, int, error) {
	model, err := s.modelService.GetChatModel(ctx, run.ReadingModelID)
	if err != nil {
		return nil, 0, fmt.Errorf("加载精读模型失败: %w", err)
	}
	notesJSON, _ := json.Marshal(notes)
	prompt := fmt.Sprintf("阅读目标：%s\n\n阅读笔记：%s\n\n所有章节已读完。请输出最终理解 JSON："+
		"{\"summary\":\"...\",\"key_points\":[],\"open_questions\":[],\"contradictions\":[],\"citations\":[]}。",
		readingGoal(run), string(notesJSON))
	resp, err := model.Chat(ctx, []chat.Message{
		{Role: "system", Content: s.readingInstructions(ctx, run)},
		{Role: "user", Content: prompt},
	}, &chat.ChatOptions{Temperature: 0.2, MaxTokens: readingMaxTokens(readingRunDepth(run))})
	if err != nil {
		return nil, 0, fmt.Errorf("精读模型调用失败: %w", err)
	}
	final := map[string]any{}
	if err := json.Unmarshal([]byte(stripJSONFence(resp.Content)), &final); err != nil {
		final = map[string]any{"summary": strings.TrimSpace(resp.Content)}
	}
	return final, resp.Usage.TotalTokens, nil
}

func readingGoal(run *types.ReadingRun) string {
	var plan types.ReadingPlan
	_ = json.Unmarshal(run.Plan, &plan)
	if strings.TrimSpace(plan.Goal) != "" {
		return plan.Goal
	}
	return "理解选定文档的核心内容、术语、矛盾与开放问题"
}

func parseReadingLLM(content string) (*readingLLMResponse, error) {
	raw := stripJSONFence(content)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("精读模型未返回 JSON")
	}
	var parsed readingLLMResponse
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("精读模型返回 JSON 无效: %w", err)
	}
	if parsed.Action == "" {
		if parsed.Question != nil && parsed.Question.Text != "" {
			parsed.Action = "ask_user"
		} else {
			parsed.Action = "update_notes"
		}
	}
	if parsed.Action != "ask_user" && parsed.Action != "update_notes" && parsed.Action != "finish" && parsed.Action != "search_selected_docs" {
		return nil, fmt.Errorf("精读模型返回非法动作: %s", parsed.Action)
	}
	if parsed.Action == "search_selected_docs" && strings.TrimSpace(parsed.SearchQuery) == "" {
		return nil, fmt.Errorf("精读检索缺少检索词")
	}
	return &parsed, nil
}

func stripJSONFence(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func mergeReadingNotes(notes *types.ReadingNotes, patch types.ReadingNotesPatch) {
	notes.Confirmed = readingAppendUnique(notes.Confirmed, patch.Confirmed...)
	notes.Terms = readingAppendUnique(notes.Terms, patch.Terms...)
	notes.OpenQuestions = readingAppendUnique(notes.OpenQuestions, patch.OpenQuestions...)
	notes.Contradictions = readingAppendUnique(notes.Contradictions, patch.Contradictions...)
	notes.Decisions = readingAppendUnique(notes.Decisions, patch.Decisions...)
	notes.Citations = readingAppendUnique(notes.Citations, patch.Citations...)
}

func readingAppendUnique(existing []string, values ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		existing = append(existing, value)
	}
	return existing
}

func readingUniqueStrings(values []string) []string {
	return readingAppendUnique(nil, values...)
}
