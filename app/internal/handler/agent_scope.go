package handler

import (
	"context"

	"gaze/internal/agent/tools"
	apperrors "gaze/internal/errors"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

// resolveAgentForRequest resolves the agent named by the agent_id query
// parameter. The personal edition has no cross-tenant agent sharing, so the
// lookup always targets the owner's own agents.
func resolveAgentForRequest(
	ctx context.Context,
	agentID string,
	agents interfaces.CustomAgentService,
) (*types.CustomAgent, error) {
	if agents == nil {
		return nil, apperrors.NewInternalServerError("Agent service is not configured")
	}
	agent, err := agents.GetAgentByID(ctx, agentID)
	if err != nil || agent == nil {
		return nil, apperrors.NewNotFoundError("agent not found")
	}
	return agent, nil
}

func filterKnowledgeByAgentScope(knowledges []*types.Knowledge, scope types.SharedAgentKBScope) []*types.Knowledge {
	filtered := make([]*types.Knowledge, 0, len(knowledges))
	for _, knowledge := range knowledges {
		if knowledge != nil && scope.Allows(knowledge.KnowledgeBaseID, knowledge.TenantID) {
			filtered = append(filtered, knowledge)
		}
	}
	return filtered
}

// filterKnowledgeBasesForSharedAgent narrows a KB list to the agent's
// configured scope. Explicit selections keep their existing behavior;
// dynamic "all" selections still respect the agent's KB capability filters.
func filterKnowledgeBasesForSharedAgent(kbs []*types.KnowledgeBase, agent *types.CustomAgent) []*types.KnowledgeBase {
	scope := types.NewSharedAgentKBScope(agent)
	filtered := make([]*types.KnowledgeBase, 0, len(kbs))
	if scope.IsEmpty() {
		return filtered
	}
	filter := tools.DeriveKBFilterForAgent(agent.Config.AgentMode, agent.Config.AllowedTools)
	for _, kb := range kbs {
		if kb == nil || !scope.Allows(kb.ID, kb.TenantID) {
			continue
		}
		if scope.IsAll() && !filter.IsEmpty() &&
			!tools.KBSatisfiesAgentRequirements(kb.Capabilities(), agent.Config.AgentMode, agent.Config.AllowedTools) {
			continue
		}
		filtered = append(filtered, kb)
	}
	return filtered
}
