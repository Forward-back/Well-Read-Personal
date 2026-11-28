package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// AgentPendingService builds L2/L3 dry-run plans, stores them as pending
// tasks, and applies them only after an explicit owner confirmation.
type agentPendingService struct {
	pendingRepo   interfaces.AgentPendingRepository
	kbService     interfaces.KnowledgeBaseService
	knowledgeRepo interfaces.KnowledgeRepository
	docs          interfaces.AgentDocumentService
	docVersions   interfaces.DocumentVersionService
}

// NewAgentPendingService wires the L2/L3 confirmation chain.
func NewAgentPendingService(
	pendingRepo interfaces.AgentPendingRepository,
	kbService interfaces.KnowledgeBaseService,
	knowledgeRepo interfaces.KnowledgeRepository,
	docs interfaces.AgentDocumentService,
	docVersions interfaces.DocumentVersionService,
) interfaces.AgentPendingService {
	return &agentPendingService{
		pendingRepo:   pendingRepo,
		kbService:     kbService,
		knowledgeRepo: knowledgeRepo,
		docs:          docs,
		docVersions:   docVersions,
	}
}

func (s *agentPendingService) Plan(
	ctx context.Context, req *types.AgentPendingPlanRequest,
) (*types.AgentPendingTask, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	userID, _ := types.UserIDFromContext(ctx)
	if userID == "" {
		userID = req.Scope.UserID
	}
	task := &types.AgentPendingTask{
		ID:                 uuid.NewString(),
		TenantID:           tenantID,
		UserID:             userID,
		SessionID:          req.Turn.SessionID,
		MessageID:          req.Turn.MessageID,
		ToolName:           req.ToolName,
		RiskLevel:          req.RiskLevel,
		Status:             types.AgentPendingStatusPending,
		TargetKnowledgeIDs: types.JSON("[]"),
		AppliedVersionIDs:  types.JSON("[]"),
		ExpiresAt:          time.Now().Add(types.AgentPendingTTL),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	var diff string
	var targets []string
	switch req.ToolName {
	case types.AgentToolSplitDocument:
		plan, err := s.planSplit(ctx, req.Scope, req.Split)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeSplitDiff(plan))
		task.BaseVersionID = plan.BaseVersionID
		targets = []string{plan.SourceID}
		diff = composeSplitDiff(plan)
	case types.AgentToolMergeDocuments:
		plan, err := s.planMerge(ctx, req.Scope, req.Merge)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeMergeDiff(plan))
		if len(plan.BaseVersions) > 0 {
			task.BaseVersionID = plan.BaseVersions[0]
		}
		targets = append([]string{}, plan.SourceIDs...)
		diff = composeMergeDiff(plan)
	case types.AgentToolDeleteDocument:
		plan, err := s.planDelete(ctx, req.Scope, req.Delete)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeTrashDiff("移入回收站", plan.Documents))
		targets = trashPlanIDs(plan.Documents)
		diff = composeTrashDiff("移入回收站", plan.Documents)
	case types.AgentToolRestoreDocument:
		plan, err := s.planRestore(ctx, req.Scope, req.Restore)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeTrashDiff("从回收站恢复", plan.Documents))
		targets = trashPlanIDs(plan.Documents)
		diff = composeTrashDiff("从回收站恢复", plan.Documents)
	case types.AgentToolOverwriteDocument:
		plan, err := s.planOverwrite(ctx, req.Scope, req.Overwrite)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeOverwriteDiff(plan))
		task.BaseVersionID = plan.BaseVersionID
		targets = []string{plan.KnowledgeID}
		diff = composeOverwriteDiff(plan)
	case types.AgentToolRevertDocument:
		plan, err := s.planRevert(ctx, req.Scope, req.Revert)
		if err != nil {
			return nil, err
		}
		task.DryRun = types.EncodePlan(req.ToolName, plan, composeRevertDiff(plan))
		targets = []string{plan.KnowledgeID}
		diff = composeRevertDiff(plan)
	default:
		return nil, werrors.NewValidationError("不支持的结构操作：" + req.ToolName)
	}
	rawTargets, _ := json.Marshal(targets)
	task.TargetKnowledgeIDs = types.JSON(rawTargets)
	if err := s.pendingRepo.Create(ctx, task); err != nil {
		return nil, err
	}
	_ = diff
	return task, nil
}

func (s *agentPendingService) Get(ctx context.Context, id string) (*types.AgentPendingView, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.ownedTask(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return pendingView(task), nil
}

func (s *agentPendingService) Confirm(
	ctx context.Context, id string, req *types.AgentConfirmRequest,
) (*types.AgentPendingExecuteResult, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.ownedTask(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if task.Status != types.AgentPendingStatusPending {
		return nil, werrors.NewValidationError("该待确认任务已结束：" + task.Status)
	}
	now := time.Now()
	if now.After(task.ExpiresAt) {
		task.Status = types.AgentPendingStatusExpired
		task.UpdatedAt = now
		_ = s.pendingRepo.Save(ctx, task)
		return nil, werrors.NewValidationError("待确认任务已过期，请让 Agent 重新预览")
	}
	// Issue and immediately consume the confirmation token: the token exists
	// for one-time audit semantics, never for client-side generation.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(hex.EncodeToString(tokenBytes)))
	task.TokenHash = hex.EncodeToString(sum[:])
	expires := now.Add(types.AgentConfirmTokenTTL)
	task.TokenExpiresAt = &expires
	task.TokenConsumedAt = &now
	if req != nil {
		task.ConfirmComment = strings.TrimSpace(req.Comment)
	}

	applied, execErr := s.execute(ctx, tenantID, task)
	task.UpdatedAt = time.Now()
	if execErr != nil {
		task.Status = types.AgentPendingStatusFailed
		task.Error = execErr.Error()
		_ = s.pendingRepo.Save(ctx, task)
		return nil, execErr
	}
	raw, _ := json.Marshal(applied)
	task.AppliedVersionIDs = types.JSON(raw)
	task.Status = types.AgentPendingStatusApplied
	if err := s.pendingRepo.Save(ctx, task); err != nil {
		return nil, err
	}
	return &types.AgentPendingExecuteResult{
		PendingID:       task.ID,
		Status:          task.Status,
		AppliedVersions: applied,
		Message:         "变更已执行并提交版本",
	}, nil
}

func (s *agentPendingService) Reject(
	ctx context.Context, id string, req *types.AgentConfirmRequest,
) (*types.AgentPendingView, error) {
	tenantID, err := writeExecutionTenant(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.ownedTask(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if task.Status == types.AgentPendingStatusPending {
		task.Status = types.AgentPendingStatusRejected
		if req != nil {
			task.ConfirmComment = strings.TrimSpace(req.Comment)
		}
		task.UpdatedAt = time.Now()
		if err := s.pendingRepo.Save(ctx, task); err != nil {
			return nil, err
		}
	}
	return pendingView(task), nil
}

func (s *agentPendingService) PurgeExpired(ctx context.Context, before time.Time) (int, error) {
	rows, err := s.pendingRepo.ListExpired(ctx, before, 200)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, task := range rows {
		task.Status = types.AgentPendingStatusExpired
		task.UpdatedAt = time.Now()
		if err := s.pendingRepo.Save(ctx, task); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// ---- execution ------------------------------------------------------------

func (s *agentPendingService) execute(
	ctx context.Context, tenantID uint64, task *types.AgentPendingTask,
) ([]string, error) {
	actor, _ := types.UserIDFromContext(ctx)
	if actor == "" {
		actor = task.UserID
	}
	switch task.ToolName {
	case types.AgentToolSplitDocument:
		var plan types.SplitPlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		if err := s.ensureCurrentVersion(ctx, tenantID, plan.SourceID, plan.BaseVersionID); err != nil {
			return nil, err
		}
		source, err := s.knowledgeRepo.GetKnowledgeByID(ctx, tenantID, plan.SourceID)
		if err != nil || source == nil {
			return nil, werrors.NewNotFoundError("源文档不存在")
		}
		extracted, err := s.docs.CreateAgentDocument(ctx, &types.AgentDocumentCreateRequest{
			KnowledgeBaseID: source.KnowledgeBaseID,
			Title:           plan.NewTitle,
			Content:         plan.NewMarkdown,
			SessionID:       task.SessionID,
			MessageID:       task.MessageID,
			ToolName:        task.ToolName,
			ActorID:         actor,
		})
		if err != nil {
			return nil, err
		}
		remaining, err := s.docs.CreateAgentDocument(ctx, &types.AgentDocumentCreateRequest{
			KnowledgeBaseID: source.KnowledgeBaseID,
			Title:           plan.RemainingTitle,
			Content:         plan.RemainingMarkdown,
			SessionID:       task.SessionID,
			MessageID:       task.MessageID,
			ToolName:        task.ToolName,
			ActorID:         actor,
		})
		if err != nil {
			return []string{extracted.VersionID}, err
		}
		if _, err := s.docs.TrashDocuments(ctx, []string{plan.SourceID}); err != nil {
			return []string{extracted.VersionID, remaining.VersionID}, err
		}
		lineage := []*types.DocLineage{
			{ID: uuid.NewString(), TenantID: tenantID, SrcKnowledgeID: plan.SourceID, DstKnowledgeID: extracted.DocumentID,
				DstVersionID: extracted.VersionID, Relation: types.DocLineageSplitFrom, PendingTaskID: task.ID, CreatedAt: time.Now()},
			{ID: uuid.NewString(), TenantID: tenantID, SrcKnowledgeID: plan.SourceID, DstKnowledgeID: remaining.DocumentID,
				DstVersionID: remaining.VersionID, Relation: types.DocLineageSplitFrom, PendingTaskID: task.ID, CreatedAt: time.Now()},
		}
		if err := s.pendingRepo.CreateLineage(ctx, lineage); err != nil {
			return []string{extracted.VersionID, remaining.VersionID}, err
		}
		return []string{extracted.VersionID, remaining.VersionID}, nil
	case types.AgentToolMergeDocuments:
		var plan types.MergePlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		for index, sourceID := range plan.SourceIDs {
			base := ""
			if index < len(plan.BaseVersions) {
				base = plan.BaseVersions[index]
			}
			if err := s.ensureCurrentVersion(ctx, tenantID, sourceID, base); err != nil {
				return nil, err
			}
		}
		first, err := s.knowledgeRepo.GetKnowledgeByID(ctx, tenantID, plan.SourceIDs[0])
		if err != nil || first == nil {
			return nil, werrors.NewNotFoundError("源文档不存在")
		}
		merged, err := s.docs.CreateAgentDocument(ctx, &types.AgentDocumentCreateRequest{
			KnowledgeBaseID: first.KnowledgeBaseID,
			Title:           plan.TargetTitle,
			Content:         plan.MergedMarkdown,
			SessionID:       task.SessionID,
			MessageID:       task.MessageID,
			ToolName:        task.ToolName,
			ActorID:         actor,
		})
		if err != nil {
			return nil, err
		}
		if _, err := s.docs.TrashDocuments(ctx, plan.SourceIDs); err != nil {
			return []string{merged.VersionID}, err
		}
		rows := make([]*types.DocLineage, 0, len(plan.SourceIDs))
		for _, sourceID := range plan.SourceIDs {
			rows = append(rows, &types.DocLineage{
				ID: uuid.NewString(), TenantID: tenantID, SrcKnowledgeID: sourceID,
				DstKnowledgeID: merged.DocumentID, DstVersionID: merged.VersionID,
				Relation: types.DocLineageMergedFrom, PendingTaskID: task.ID, CreatedAt: time.Now(),
			})
		}
		if err := s.pendingRepo.CreateLineage(ctx, rows); err != nil {
			return []string{merged.VersionID}, err
		}
		return []string{merged.VersionID}, nil
	case types.AgentToolDeleteDocument:
		var plan types.DeletePlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		if _, err := s.docs.TrashDocuments(ctx, trashPlanIDs(plan.Documents)); err != nil {
			return nil, err
		}
		return nil, nil
	case types.AgentToolRestoreDocument:
		var plan types.RestorePlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		if _, err := s.docs.RestoreDocuments(ctx, trashPlanIDs(plan.Documents)); err != nil {
			return nil, err
		}
		return nil, nil
	case types.AgentToolOverwriteDocument:
		var plan types.OverwritePlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		knowledge, err := s.knowledgeRepo.GetKnowledgeByID(ctx, tenantID, plan.KnowledgeID)
		if err != nil || knowledge == nil {
			return nil, werrors.NewNotFoundError("文档不存在")
		}
		if err := s.ensureCurrentVersion(ctx, tenantID, plan.KnowledgeID, plan.BaseVersionID); err != nil {
			return nil, err
		}
		version, err := s.docs.CommitAgentMarkdown(ctx, &types.AgentMarkdownCommitRequest{
			KnowledgeID:       plan.KnowledgeID,
			ActorID:           actor,
			SessionID:         task.SessionID,
			MessageID:         task.MessageID,
			ToolName:          task.ToolName,
			PendingTaskID:     task.ID,
			SourceType:        types.DocVersionSourceAgentEdit,
			Title:             "Agent 全文覆盖：" + knowledge.Title,
			Description:       "确认后执行 overwrite_document",
			Markdown:          plan.NewMarkdown,
			ExpectedVersionID: plan.BaseVersionID,
		})
		if err != nil {
			return nil, err
		}
		return []string{version.ID}, nil
	case types.AgentToolRevertDocument:
		var plan types.RevertPlan
		if err := decodePlan(task, &plan); err != nil {
			return nil, err
		}
		knowledge, err := s.knowledgeRepo.GetKnowledgeByID(ctx, tenantID, plan.KnowledgeID)
		if err != nil || knowledge == nil {
			return nil, werrors.NewNotFoundError("文档不存在")
		}
		version, err := s.docVersions.Rollback(ctx, tenantID, knowledge.KnowledgeBaseID, knowledge.ID,
			plan.TargetVersionID, actor, fmt.Sprintf("Agent 回滚（pending %s）", task.ID))
		if err != nil {
			return nil, err
		}
		return []string{version.ID}, nil
	default:
		return nil, werrors.NewValidationError("不支持的结构操作：" + task.ToolName)
	}
}

func (s *agentPendingService) ensureCurrentVersion(
	ctx context.Context, tenantID uint64, knowledgeID, baseVersionID string,
) error {
	if strings.TrimSpace(baseVersionID) == "" {
		return nil
	}
	latest, err := s.docVersions.LatestVersion(ctx, tenantID, knowledgeID)
	if err != nil {
		return err
	}
	if latest == nil || latest.ID != baseVersionID {
		return werrors.NewConflictError("文档已被修改，请让 Agent 重新预览")
	}
	return nil
}

// ---- plan builders --------------------------------------------------------

func (s *agentPendingService) ensureInScope(
	ctx context.Context, scope types.AgentWorkspaceScope, knowledgeID string,
) (*types.Knowledge, error) {
	knowledge, err := s.knowledgeRepo.GetKnowledgeByID(ctx, scope.TenantID, knowledgeID)
	if err != nil || knowledge == nil {
		return nil, werrors.NewNotFoundError("文档不存在：" + knowledgeID)
	}
	if !wsContainsString(scope.KnowledgeBaseIDs, knowledge.KnowledgeBaseID) {
		return nil, werrors.NewValidationError("文档不在会话范围内")
	}
	if len(scope.KnowledgeIDs) > 0 && !wsContainsString(scope.KnowledgeIDs, knowledgeID) {
		return nil, werrors.NewValidationError("文档不在会话范围内")
	}
	return knowledge, nil
}

func (s *agentPendingService) planSplit(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.SplitDocumentArgs,
) (*types.SplitPlan, error) {
	if args == nil || strings.TrimSpace(args.KnowledgeID) == "" || strings.TrimSpace(args.Heading) == "" {
		return nil, werrors.NewValidationError("缺少 knowledge_id 或 heading")
	}
	knowledge, err := s.ensureInScope(ctx, scope, args.KnowledgeID)
	if err != nil {
		return nil, err
	}
	view, err := s.docs.GetDocumentContent(ctx, knowledge.ID, "")
	if err != nil {
		return nil, err
	}
	section, _, err := locateMarkdownSection(view.Markdown, args.Heading, args.HeadingLevel, 1)
	if err != nil {
		return nil, werrors.NewValidationError(err.Error())
	}
	runes := []rune(view.Markdown)
	sectionText := strings.TrimSpace(string(runes[section.Start:section.End]))
	remaining := strings.TrimSpace(string(runes[:section.Start]) + "\n\n" + string(runes[section.End:]))
	if sectionText == "" || remaining == "" {
		return nil, werrors.NewValidationError("拆分后两侧都必须有内容")
	}
	newTitle := strings.TrimSpace(args.NewTitle)
	if newTitle == "" {
		newTitle = knowledge.Title + "·拆分"
	}
	baseID, baseLabel := "", ""
	if view.Version != nil {
		baseID = view.Version.ID
		baseLabel = view.Version.VersionLabel
	}
	return &types.SplitPlan{
		SourceID: knowledge.ID, SourceTitle: knowledge.Title,
		Heading: args.Heading, NewTitle: newTitle,
		RemainingTitle:    knowledge.Title + "·其余",
		NewMarkdown:       finalizeMarkdown(sectionText),
		RemainingMarkdown: finalizeMarkdown(remaining),
		BaseVersionID:     baseID,
		BaseVersionLabel:  baseLabel,
	}, nil
}

func (s *agentPendingService) planMerge(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.MergeDocumentsArgs,
) (*types.MergePlan, error) {
	if args == nil || len(args.KnowledgeIDs) < 2 {
		return nil, werrors.NewValidationError("merge_documents 至少需要两个文档")
	}
	ids := wsUniqueStrings(args.KnowledgeIDs)
	titles := make([]string, 0, len(ids))
	contents := make([]string, 0, len(ids))
	baseVersions := make([]string, 0, len(ids))
	kbID := ""
	for _, id := range ids {
		knowledge, err := s.ensureInScope(ctx, scope, id)
		if err != nil {
			return nil, err
		}
		if kbID == "" {
			kbID = knowledge.KnowledgeBaseID
		} else if knowledge.KnowledgeBaseID != kbID {
			return nil, werrors.NewValidationError("仅支持合并同一知识库内的文档")
		}
		view, err := s.docs.GetDocumentContent(ctx, id, "")
		if err != nil {
			return nil, err
		}
		titles = append(titles, knowledge.Title)
		contents = append(contents, strings.TrimSpace(view.Markdown))
		if view.Version != nil {
			baseVersions = append(baseVersions, view.Version.ID)
		} else {
			baseVersions = append(baseVersions, "")
		}
	}
	merged := finalizeMarkdown(strings.Join(contents, "\n\n---\n\n"))
	if len([]rune(merged)) > manualContentMaxLength {
		return nil, werrors.NewValidationError("合并后文档超过长度限制")
	}
	title := strings.TrimSpace(args.Title)
	if title == "" {
		title = titles[0] + "（合并）"
	}
	return &types.MergePlan{
		TargetTitle: title, SourceIDs: ids, SourceTitles: titles,
		MergedMarkdown: merged, BaseVersions: baseVersions,
	}, nil
}

func (s *agentPendingService) planDelete(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.DeleteDocumentsArgs,
) (*types.DeletePlan, error) {
	if args == nil || len(args.KnowledgeIDs) == 0 {
		return nil, werrors.NewValidationError("delete_document 缺少文档")
	}
	docs := make([]types.TrashPlanDocument, 0, len(args.KnowledgeIDs))
	for _, id := range wsUniqueStrings(args.KnowledgeIDs) {
		knowledge, err := s.ensureInScope(ctx, scope, id)
		if err != nil {
			return nil, err
		}
		docs = append(docs, types.TrashPlanDocument{ID: knowledge.ID, Title: knowledge.Title})
	}
	return &types.DeletePlan{Documents: docs}, nil
}

func (s *agentPendingService) planRestore(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.RestoreDocumentsArgs,
) (*types.RestorePlan, error) {
	if args == nil || len(args.KnowledgeIDs) == 0 {
		return nil, werrors.NewValidationError("restore_document 缺少文档")
	}
	docs := make([]types.TrashPlanDocument, 0, len(args.KnowledgeIDs))
	for _, id := range wsUniqueStrings(args.KnowledgeIDs) {
		row, err := s.knowledgeRepo.GetKnowledgeUnscoped(ctx, scope.TenantID, id)
		if err != nil || row == nil || !row.DeletedAt.Valid {
			return nil, werrors.NewValidationError("仅回收站中的文档可以恢复：" + id)
		}
		if !wsContainsString(scope.KnowledgeBaseIDs, row.KnowledgeBaseID) {
			return nil, werrors.NewValidationError("文档不在会话范围内")
		}
		docs = append(docs, types.TrashPlanDocument{ID: row.ID, Title: row.Title})
	}
	return &types.RestorePlan{Documents: docs}, nil
}

func (s *agentPendingService) planOverwrite(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.OverwriteDocumentArgs,
) (*types.OverwritePlan, error) {
	if args == nil || strings.TrimSpace(args.KnowledgeID) == "" {
		return nil, werrors.NewValidationError("缺少 knowledge_id")
	}
	content := strings.TrimSpace(args.Content)
	if content == "" {
		return nil, werrors.NewValidationError("overwrite_document 需要完整正文")
	}
	if len([]rune(content)) > manualContentMaxLength {
		return nil, werrors.NewValidationError("文档超过长度限制")
	}
	knowledge, err := s.ensureInScope(ctx, scope, args.KnowledgeID)
	if err != nil {
		return nil, err
	}
	view, err := s.docs.GetDocumentContent(ctx, knowledge.ID, "")
	if err != nil {
		return nil, err
	}
	added, removed := estimateChangedLines(view.Markdown, content)
	baseID, baseLabel := "", ""
	if view.Version != nil {
		baseID = view.Version.ID
		baseLabel = view.Version.VersionLabel
	}
	return &types.OverwritePlan{
		KnowledgeID: knowledge.ID, Title: knowledge.Title,
		NewMarkdown: finalizeMarkdown(content),
		BaseVersionID: baseID, BaseVersionLabel: baseLabel,
		AddedLines: added, RemovedLines: removed,
	}, nil
}

func (s *agentPendingService) planRevert(
	ctx context.Context, scope types.AgentWorkspaceScope, args *types.RevertDocumentArgs,
) (*types.RevertPlan, error) {
	if args == nil || strings.TrimSpace(args.KnowledgeID) == "" || strings.TrimSpace(args.TargetVersionID) == "" {
		return nil, werrors.NewValidationError("缺少 knowledge_id 或 target_version_id")
	}
	knowledge, err := s.ensureInScope(ctx, scope, args.KnowledgeID)
	if err != nil {
		return nil, err
	}
	version, chunks, err := s.docVersions.GetVersion(ctx, scope.TenantID, knowledge.KnowledgeBaseID, knowledge.ID, args.TargetVersionID)
	if err != nil || version == nil {
		return nil, werrors.NewNotFoundError("目标版本不存在")
	}
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return &types.RevertPlan{
		KnowledgeID: knowledge.ID, Title: knowledge.Title,
		TargetVersionID: version.ID, TargetVersionLabel: version.VersionLabel,
		TargetMarkdown: strings.Join(parts, "\n\n"),
	}, nil
}

// ---- helpers --------------------------------------------------------------

func (s *agentPendingService) ownedTask(
	ctx context.Context, tenantID uint64, id string,
) (*types.AgentPendingTask, error) {
	task, err := s.pendingRepo.Get(ctx, tenantID, id)
	if err != nil || task == nil {
		return nil, werrors.NewNotFoundError("待确认任务不存在")
	}
	if userID, ok := types.UserIDFromContext(ctx); ok && userID != "" && task.UserID != "" && task.UserID != userID {
		return nil, werrors.NewNotFoundError("待确认任务不存在")
	}
	return task, nil
}

func pendingView(task *types.AgentPendingTask) *types.AgentPendingView {
	view := &types.AgentPendingView{
		ID: task.ID, SessionID: task.SessionID, MessageID: task.MessageID,
		ToolName: task.ToolName, RiskLevel: task.RiskLevel, Status: task.Status,
		ExpiresAt: task.ExpiresAt, Error: task.Error,
	}
	var dryRun map[string]interface{}
	if len(task.DryRun) > 0 {
		_ = json.Unmarshal(task.DryRun, &dryRun)
	}
	view.DryRun = dryRun
	var applied []string
	if len(task.AppliedVersionIDs) > 0 {
		_ = json.Unmarshal(task.AppliedVersionIDs, &applied)
	}
	view.AppliedVersions = applied
	return view
}

func decodePlan(task *types.AgentPendingTask, target interface{}) error {
	var wrapper struct {
		Plan json.RawMessage `json:"plan"`
	}
	if err := json.Unmarshal(task.DryRun, &wrapper); err != nil {
		return werrors.NewValidationError("预览数据损坏，请重新预览")
	}
	if len(wrapper.Plan) == 0 {
		return werrors.NewValidationError("预览数据缺失，请重新预览")
	}
	if err := json.Unmarshal(wrapper.Plan, target); err != nil {
		return werrors.NewValidationError("预览数据损坏，请重新预览")
	}
	return nil
}

func trashPlanIDs(docs []types.TrashPlanDocument) []string {
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func composeSplitDiff(plan *types.SplitPlan) string {
	return fmt.Sprintf(
		"拆分《%s》：提取「%s」到新文档《%s》（%d 字），其余保留为新文档《%s》（%d 字）；原文档移入回收站（保留 %d 天）。",
		plan.SourceTitle, plan.Heading, plan.NewTitle, len([]rune(plan.NewMarkdown)),
		plan.RemainingTitle, len([]rune(plan.RemainingMarkdown)), types.KnowledgeTrashKeepDays,
	)
}

func composeMergeDiff(plan *types.MergePlan) string {
	return fmt.Sprintf(
		"合并 %d 篇文档（%s）为新文档《%s》（%d 字）；源文档全部移入回收站（保留 %d 天）。",
		len(plan.SourceIDs), strings.Join(plan.SourceTitles, "、"),
		plan.TargetTitle, len([]rune(plan.MergedMarkdown)), types.KnowledgeTrashKeepDays,
	)
}

func composeTrashDiff(action string, docs []types.TrashPlanDocument) string {
	titles := make([]string, 0, len(docs))
	for _, doc := range docs {
		titles = append(titles, doc.Title)
	}
	return fmt.Sprintf("%s：%s。回收站保留 %d 天。", action, strings.Join(titles, "、"), types.KnowledgeTrashKeepDays)
}

func composeOverwriteDiff(plan *types.OverwritePlan) string {
	return fmt.Sprintf("全文覆盖《%s》：+%d 行 / -%d 行，基于版本 %s。", plan.Title, plan.AddedLines, plan.RemovedLines, plan.BaseVersionLabel)
}

func composeRevertDiff(plan *types.RevertPlan) string {
	return fmt.Sprintf("回滚《%s》到版本 %s（%d 字），生成新的回滚版本。", plan.Title, plan.TargetVersionLabel, len([]rune(plan.TargetMarkdown)))
}
