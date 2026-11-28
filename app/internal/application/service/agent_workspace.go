package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gaze/internal/agent"
	agenttools "gaze/internal/agent/tools"
	appconfig "gaze/internal/config"
	"gaze/internal/engine"
	"gaze/internal/event"
	"gaze/internal/logger"
	"gaze/internal/models/chat"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	werrors "gaze/internal/errors"
)

const (
	agentWorkspaceMaxEditsPerTurn = 3
	agentWorkspaceHistoryTurns    = 8
	agentWorkspaceToolOutputChars = 60000
)

type agentWorkspaceService struct {
	cfg           *appconfig.Config
	sessionRepo   interfaces.SessionRepository
	messageRepo   interfaces.MessageRepository
	kbService     interfaces.KnowledgeBaseService
	knowledgeRepo interfaces.KnowledgeRepository
	agentDocs     interfaces.AgentDocumentService
	pending       interfaces.AgentPendingService
	modelService  interfaces.ModelService
	ragEngine     engine.RAGEngine
	docVersions   interfaces.DocumentVersionService
}

// NewAgentWorkspaceService creates the Agent-workspace session and run service.
func NewAgentWorkspaceService(
	cfg *appconfig.Config,
	sessionRepo interfaces.SessionRepository,
	messageRepo interfaces.MessageRepository,
	kbService interfaces.KnowledgeBaseService,
	knowledgeRepo interfaces.KnowledgeRepository,
	agentDocs interfaces.AgentDocumentService,
	pending interfaces.AgentPendingService,
	modelService interfaces.ModelService,
	ragEngine engine.RAGEngine,
	docVersions interfaces.DocumentVersionService,
) interfaces.AgentWorkspaceService {
	return &agentWorkspaceService{
		cfg:           cfg,
		sessionRepo:   sessionRepo,
		messageRepo:   messageRepo,
		kbService:     kbService,
		knowledgeRepo: knowledgeRepo,
		agentDocs:     agentDocs,
		pending:       pending,
		modelService:  modelService,
		ragEngine:     ragEngine,
		docVersions:   docVersions,
	}
}

func workspaceCaller(ctx context.Context) (uint64, string, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return 0, "", werrors.NewUnauthorizedError("未登录")
	}
	userID, _ := types.UserIDFromContext(ctx)
	return tenantID, userID, nil
}

func (s *agentWorkspaceService) CreateSession(
	ctx context.Context, req *types.CreateAgentWorkspaceSessionRequest,
) (*types.AgentWorkspaceSessionView, error) {
	tenantID, userID, err := workspaceCaller(ctx)
	if err != nil {
		return nil, err
	}
	kbIDs, knowledgeIDs, modelID, err := s.validateSessionScope(ctx, tenantID, req.KnowledgeBaseIDs, req.KnowledgeIDs, req.ModelID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Agent 任务"
	}
	state := &types.SessionLastRequestState{
		ModelID:          modelID,
		KnowledgeBaseIDs: kbIDs,
		KnowledgeIDs:     knowledgeIDs,
	}
	session := &types.Session{
		ID:               uuid.NewString(),
		TenantID:         tenantID,
		UserID:           userID,
		Title:            title,
		Surface:          types.AgentWorkspaceSurface,
		SummaryModelID:   modelID,
		LastRequestState: state,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if _, err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}
	return sessionView(session), nil
}

func (s *agentWorkspaceService) ListSessions(
	ctx context.Context, page *types.Pagination, keyword string,
) ([]*types.AgentWorkspaceSessionView, int64, error) {
	tenantID, userID, err := workspaceCaller(ctx)
	if err != nil {
		return nil, 0, err
	}
	if page == nil {
		page = &types.Pagination{Page: 1, PageSize: 20}
	}
	items, total, err := s.sessionRepo.QueryPaged(ctx, &types.SessionListQuery{
		TenantID: tenantID,
		UserID:   userID,
		Keyword:  keyword,
		Surface:  types.AgentWorkspaceSurface,
		Page:     page.GetPage(),
		PageSize: page.GetPageSize(),
	})
	if err != nil {
		return nil, 0, err
	}
	views := make([]*types.AgentWorkspaceSessionView, 0, len(items))
	for _, item := range items {
		views = append(views, sessionView(&item.Session))
	}
	return views, total, nil
}

func (s *agentWorkspaceService) GetSession(
	ctx context.Context, id string,
) (*types.AgentWorkspaceSessionView, error) {
	session, err := s.ownedSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return sessionView(session), nil
}

func (s *agentWorkspaceService) UpdateSession(
	ctx context.Context, id string, req *types.UpdateAgentWorkspaceSessionRequest,
) (*types.AgentWorkspaceSessionView, error) {
	session, err := s.ownedSession(ctx, id)
	if err != nil {
		return nil, err
	}
	tenantID, userID, _ := workspaceCaller(ctx)
	state := session.LastRequestState
	if state == nil {
		state = &types.SessionLastRequestState{}
	}
	title := ""
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	modelID := session.SummaryModelID
	if req.ModelID != nil && strings.TrimSpace(*req.ModelID) != "" {
		kbIDs, knowledgeIDs, validatedModel, err := s.validateSessionScope(
			ctx, tenantID, stateKnowledgeBaseIDs(state), state.KnowledgeIDs, *req.ModelID,
		)
		if err != nil {
			return nil, err
		}
		state.KnowledgeBaseIDs = kbIDs
		state.KnowledgeIDs = knowledgeIDs
		modelID = validatedModel
	}
	if len(req.KnowledgeBaseIDs) > 0 {
		kbIDs, knowledgeIDs, validatedModel, err := s.validateSessionScope(
			ctx, tenantID, req.KnowledgeBaseIDs, req.KnowledgeIDs, modelID,
		)
		if err != nil {
			return nil, err
		}
		state.KnowledgeBaseIDs = kbIDs
		state.KnowledgeIDs = knowledgeIDs
		modelID = validatedModel
	}
	state.ModelID = modelID
	if _, err := s.sessionRepo.UpdateAgentWorkspace(ctx, tenantID, userID, id, title, modelID, state); err != nil {
		return nil, err
	}
	refreshed, err := s.ownedSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return sessionView(refreshed), nil
}

func (s *agentWorkspaceService) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.ownedSession(ctx, id); err != nil {
		return err
	}
	tenantID, userID, _ := workspaceCaller(ctx)
	_, err := s.sessionRepo.Delete(ctx, tenantID, userID, id)
	return err
}

func (s *agentWorkspaceService) ListMessages(
	ctx context.Context, id string, page, pageSize int,
) ([]*types.Message, error) {
	if _, err := s.ownedSession(ctx, id); err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 100
	}
	return s.messageRepo.GetMessagesBySession(ctx, id, page, pageSize)
}

// RunTurn drives one workspace conversation turn on the shared agent runtime.
func (s *agentWorkspaceService) RunTurn(
	ctx context.Context, session *types.Session, assistantMessageID, query string, bus *event.EventBus,
) error {
	if session == nil || session.Surface != types.AgentWorkspaceSurface {
		return werrors.NewNotFoundError("会话不存在")
	}
	if strings.TrimSpace(query) == "" {
		return werrors.NewValidationError("问题不能为空")
	}
	state := session.LastRequestState
	if state == nil || len(state.KnowledgeBaseIDs) == 0 {
		return werrors.NewValidationError("会话缺少知识库范围")
	}
	modelID := strings.TrimSpace(session.SummaryModelID)
	if modelID == "" {
		modelID = strings.TrimSpace(state.ModelID)
	}
	if modelID == "" {
		return werrors.NewValidationError("会话未选择模型")
	}
	chatModel, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		return fmt.Errorf("加载模型失败：%w", err)
	}
	modelContextWindow := 0
	if info, err := s.modelService.GetModelByID(ctx, modelID); err == nil && info != nil {
		modelContextWindow = info.Parameters.ContextWindow
	}
	kbInfos := make([]*agent.KnowledgeBaseInfo, 0, len(state.KnowledgeBaseIDs))
	for _, kbID := range state.KnowledgeBaseIDs {
		kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
		if err != nil || kb == nil {
			continue
		}
		kbInfos = append(kbInfos, &agent.KnowledgeBaseInfo{
			ID:           kb.ID,
			Name:         kb.Name,
			Type:         kb.Type,
			Description:  kb.Description,
			Capabilities: []string{"chunks"},
		})
	}
	citationDisabled := false
	agentCfg := &types.AgentConfig{
		MaxIterations:      12,
		AllowedTools:       types.AgentWorkspaceToolNames,
		Temperature:        0.2,
		KnowledgeBases:     state.KnowledgeBaseIDs,
		KnowledgeIDs:       state.KnowledgeIDs,
		UseCustomSystemPrompt: true,
		SystemPrompt:       workspaceAgentSystemPrompt,
		MultiTurnEnabled:   true,
		HistoryTurns:       agentWorkspaceHistoryTurns,
		CitationEnabled:    &citationDisabled,
		MaxToolOutputChars: agentWorkspaceToolOutputChars,
		MaxContextTokens:   types.AgentMaxContextTokens(0, modelContextWindow),
	}

	turn := &types.AgentWorkspaceTurnContext{
		SessionID:   session.ID,
		MessageID:   assistantMessageID,
		ChatModelID: modelID,
	}
	scope := types.AgentWorkspaceScope{
		TenantID:         session.TenantID,
		UserID:           session.UserID,
		KnowledgeBaseIDs: state.KnowledgeBaseIDs,
		KnowledgeIDs:     state.KnowledgeIDs,
	}
	backend := &workspaceTurnBackend{svc: s, scope: scope, turn: turn, chatModel: chatModel}
	registry := agenttools.NewToolRegistry()
	registry.SetMaxToolOutputSize(agentWorkspaceToolOutputChars)
	agenttools.RegisterWorkspaceTools(registry, agenttools.NewWorkspaceTools(backend, scope, turn))

	engineInstance := agent.NewAgentEngine(
		agentCfg, chatModel, registry, bus, kbInfos, nil, session.ID, workspaceAgentSystemPrompt,
	)
	if engineInstance == nil {
		return fmt.Errorf("Agent 引擎初始化失败")
	}
	engineInstance.SetAppConfig(s.cfg)

	history, err := LoadAgentHistory(ctx, s.messageRepo, session.ID, agentWorkspaceHistoryTurns)
	if err != nil {
		logger.Warnf(ctx, "agent workspace: load history failed: %v", err)
		history = []chat.Message{}
	}
	if _, err := engineInstance.Execute(ctx, session.ID, assistantMessageID, query, history); err != nil {
		return err
	}
	return nil
}

func (s *agentWorkspaceService) ownedSession(ctx context.Context, id string) (*types.Session, error) {
	tenantID, userID, err := workspaceCaller(ctx)
	if err != nil {
		return nil, err
	}
	session, err := s.sessionRepo.Get(ctx, tenantID, userID, id)
	if err != nil || session == nil || session.Surface != types.AgentWorkspaceSurface {
		return nil, werrors.NewNotFoundError("会话不存在")
	}
	return session, nil
}

func (s *agentWorkspaceService) validateSessionScope(
	ctx context.Context, tenantID uint64, kbIDs, knowledgeIDs []string, modelID string,
) ([]string, []string, string, error) {
	kbs := wsUniqueStrings(kbIDs)
	if len(kbs) == 0 {
		return nil, nil, "", werrors.NewValidationError("请至少选择一个知识库")
	}
	for _, kbID := range kbs {
		kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
		if err != nil || kb == nil || kb.TenantID != tenantID {
			return nil, nil, "", werrors.NewValidationError("知识库不存在：" + kbID)
		}
		if kb.Type == types.KnowledgeBaseTypeFAQ {
			return nil, nil, "", werrors.NewValidationError("Agent 工作区暂不支持 FAQ 知识库")
		}
	}
	docs := wsUniqueStrings(knowledgeIDs)
	if len(docs) > 0 {
		rows, err := s.knowledgeRepo.GetKnowledgeBatch(ctx, tenantID, docs)
		if err != nil || len(rows) != len(docs) {
			return nil, nil, "", werrors.NewValidationError("部分文档不存在")
		}
		allowed := map[string]bool{}
		for _, kbID := range kbs {
			allowed[kbID] = true
		}
		for _, row := range rows {
			if !allowed[row.KnowledgeBaseID] {
				return nil, nil, "", werrors.NewValidationError("文档不在所选知识库范围内：" + row.ID)
			}
		}
	}
	validatedModel := strings.TrimSpace(modelID)
	if validatedModel != "" {
		model, err := s.modelService.GetModelByID(ctx, validatedModel)
		if err != nil || model == nil {
			return nil, nil, "", werrors.NewValidationError("模型不存在：" + validatedModel)
		}
		if model.Type != types.ModelTypeKnowledgeQA {
			return nil, nil, "", werrors.NewValidationError("请选择知识问答模型")
		}
	} else {
		models, err := s.modelService.ListModels(ctx)
		if err != nil {
			return nil, nil, "", err
		}
		for _, model := range models {
			if model.Type == types.ModelTypeKnowledgeQA {
				validatedModel = model.ID
				break
			}
		}
		if validatedModel == "" {
			return nil, nil, "", werrors.NewValidationError("请先在模型设置中配置知识问答模型")
		}
	}
	return kbs, docs, validatedModel, nil
}

func sessionView(session *types.Session) *types.AgentWorkspaceSessionView {
	view := &types.AgentWorkspaceSessionView{
		ID:        session.ID,
		Title:     session.Title,
		Surface:   session.Surface,
		ModelID:   session.SummaryModelID,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	if state := session.LastRequestState; state != nil {
		view.KnowledgeBaseIDs = state.KnowledgeBaseIDs
		view.KnowledgeIDs = state.KnowledgeIDs
		if view.ModelID == "" {
			view.ModelID = state.ModelID
		}
	}
	return view
}

func stateKnowledgeBaseIDs(state *types.SessionLastRequestState) []string {
	if state == nil {
		return nil
	}
	return state.KnowledgeBaseIDs
}

func wsUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func wsContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
