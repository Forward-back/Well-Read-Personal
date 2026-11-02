package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"gaze/internal/event"
	"gaze/internal/logger"
	"gaze/internal/types"
)

const maxSkillInstructionsRunes = 32000

// resolveSkillInjection validates the selected tenant skill and freezes the
// instructions + content hash that this turn injects.
func (s *sessionService) resolveSkillInjection(
	ctx context.Context, tenantID uint64, skillID string,
) (*types.SkillInjection, *types.MessageSkill, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil, nil, nil
	}
	if s.tenantSkillRepo == nil {
		return nil, nil, fmt.Errorf("skill catalog is not configured")
	}
	entity, err := s.tenantSkillRepo.GetCatalog(ctx, tenantID, skillID)
	if err != nil {
		return nil, nil, err
	}
	if entity == nil || !entity.Enabled {
		return nil, nil, fmt.Errorf("selected skill is unavailable")
	}
	instructions := strings.TrimSpace(entity.Instructions)
	if instructions == "" {
		return nil, nil, fmt.Errorf("selected skill has no instructions")
	}
	if len([]rune(instructions)) > maxSkillInstructionsRunes {
		return nil, nil, fmt.Errorf("selected skill instructions exceed the size limit")
	}
	sum := sha256.Sum256([]byte(instructions))
	hash := hex.EncodeToString(sum[:])
	injection := &types.SkillInjection{
		ID:           entity.ID,
		Name:         entity.Name,
		Version:      entity.Version,
		ContentHash:  hash,
		Instructions: instructions,
	}
	meta := &types.MessageSkill{
		ID:              entity.ID,
		Name:            entity.Name,
		Version:         entity.Version,
		ContentHash:     hash,
		InjectionStatus: "applied",
	}
	return injection, meta, nil
}

// KnowledgeQAFusion runs the knowledge QA turn through the external YLSW
// retrieval/generation engine (银联商务 RAG fusion). It streams the answer
// back through the event bus so the existing SSE output and message
// persistence paths stay unchanged.
func (s *sessionService) KnowledgeQAFusion(
	ctx context.Context, qaReq *types.QARequest, eventBus *event.EventBus,
) error {
	sessionID := qaReq.Session.ID
	question := qaReq.Query

	kbID := ""
	if len(qaReq.KnowledgeBaseIDs) > 0 {
		kbID = qaReq.KnowledgeBaseIDs[0]
	}

	logger.Infof(ctx, "YLSW fusion QA started, session: %s, kb: %s", sessionID, kbID)

	// 所有 answer 增量共享同一事件 ID（与原版管线一致）：前端按 event_id
	// 累积渲染单个答案事件，若每个 delta 使用唯一 ID 会破坏打字机/累积渲染。
	answerID := generateEventID("answer")

	emit := func(t event.EventType, id string, data interface{}) {
		if eventBus == nil {
			return
		}
		if err := eventBus.Emit(ctx, event.Event{
			ID:        id,
			Type:      t,
			SessionID: sessionID,
			Data:      data,
		}); err != nil {
			logger.Errorf(ctx, "Failed to emit %s event: %v", t, err)
		}
	}

	// 指代消解（需求五）：仅 Web 快速问答开启；失败/超时原样透传。
	// 事件在检索前发出，前端把「已改写为」挂到本轮 user 消息上。
	if rewrite := s.maybeRewriteQuery(ctx, qaReq); rewrite != nil && rewrite.Applied {
		question = rewrite.Rewritten
		emit(event.EventQueryRewritten, generateEventID("query-rewritten"), event.QueryData{
			OriginalQuery:  rewrite.Original,
			RewrittenQuery: rewrite.Rewritten,
			SessionID:      sessionID,
			UserMessageID:  qaReq.UserMessageID,
			Extra: map[string]interface{}{
				"t_ms":         rewrite.Elapsed.Milliseconds(),
				"trigger":      rewrite.Trigger,
				"model_id":     rewrite.ModelID,
				"tokens":       rewrite.Tokens,
				"rewrite_meta": rewrite.Meta,
			},
		})
	}

	if len(qaReq.KnowledgeBaseIDs) == 0 {
		return fmt.Errorf("fusion QA failed: no knowledge base selected")
	}
	skillInjection, skillMeta, err := s.resolveSkillInjection(ctx, qaReq.Session.TenantID, qaReq.SkillID)
	if err != nil {
		return fmt.Errorf("fusion QA failed: resolve skill: %w", err)
	}

	answer, refs, err := s.ragEngine.StreamChat(ctx, question, qaReq.KnowledgeBaseIDs,
		skillInjection, func(delta string) {
			emit(event.EventAgentFinalAnswer, answerID, event.AgentFinalAnswerData{
				Content: delta,
				Done:    false,
			})
		}, func(r []*types.SearchResult) {
			// 检索完成即发 references（对齐原版 pre-answer 事件顺序）
			emit(event.EventAgentReferences, generateEventID("references"), event.AgentReferencesData{
				References: r,
				Iteration:  1,
			})
		}, func(stage string, counts map[string]int, tMs int64, detail map[string]interface{}) {
			if stage == "skill_injected" && skillMeta != nil {
				if status, _ := detail["status"].(string); status != "" {
					skillMeta.InjectionStatus = status
				}
			}
			// 检索阶段状态透传给前端状态栏 + 时间线（Complete 时随消息落库）
			emit(event.EventAgentStage, generateEventID("stage"), event.AgentStageData{
				Stage:  stage,
				TMs:    tMs,
				Counts: counts,
				Detail: detail,
			})
		})
	if err != nil {
		logger.Errorf(ctx, "YLSW fusion QA failed: %v", err)
		return fmt.Errorf("fusion QA failed: %w", err)
	}

	// 引用面板展示「所属知识库」：YLSW 只回 KB id，这里补名称。
	// 优先用每条引用自带的 knowledge_base_id（会话可能没显式传 KB），
	// 取不到再退回请求里的 kbID。
	kbNameCache := map[string]string{}
	for _, r := range refs {
		if r == nil {
			continue
		}
		refKBID := r.KnowledgeBaseID
		if refKBID == "" {
			refKBID = kbID
		}
		if refKBID == "" {
			continue
		}
		name, ok := kbNameCache[refKBID]
		if !ok {
			if kb, kbErr := s.knowledgeBaseService.GetKnowledgeBaseByID(ctx, refKBID); kbErr == nil && kb != nil {
				name = kb.Name
			}
			kbNameCache[refKBID] = name
		}
		if name == "" {
			continue
		}
		if r.Metadata == nil {
			r.Metadata = map[string]string{}
		}
		r.Metadata["kb_name"] = name
	}

	// 引用绑定当前版本：回答落库后仍能说明“当时依据的是哪一版”。
	AttachReferenceVersions(ctx, s.docVersions, qaReq.Session.TenantID, refs)

	// 用携带内联位置（start_at/end_at）的最终引用覆盖一次，必须在
	// Done 事件之前发：qa.go 在 Done=true 时落库 assistantMessage，
	// 落库前 upsert 到位的引用才能带位置持久化。前端按 replace 语义
	// 覆盖，后端按 chunk ID upsert，不会重复累积。
	if len(refs) > 0 {
		emit(event.EventAgentReferences, generateEventID("references"), event.AgentReferencesData{
			References: refs,
			Iteration:  1,
		})
	}

	// Terminate the answer stream (per-chunk events above already carried the text).
	// EventAgentComplete 由 qa.go 的 final_answer 监听器在 Done=true 时统一发出，
	// 此处不再重复发（避免前端收到两个 complete）。
	emit(event.EventAgentFinalAnswer, answerID, event.AgentFinalAnswerData{
		Content: "",
		Done:    true,
		Skill:   skillMeta,
	})

	logger.Infof(ctx, "YLSW fusion QA completed, session: %s, answer chars: %d, refs: %d",
		sessionID, len(answer), len(refs))
	return nil
}
