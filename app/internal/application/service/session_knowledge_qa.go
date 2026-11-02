package service

import (
	"context"
	"fmt"

	"gaze/internal/agent/tools"
	"gaze/internal/logger"
	"gaze/internal/types"
)

// selectChatModelID selects the appropriate chat model ID with priority for Remote models
// Priority order:
// 1. Session's SummaryModelID if it's a Remote model
// 2. First knowledge base with a Remote model (from knowledgeBaseIDs or derived from knowledgeIDs)
// 3. Session's SummaryModelID (if not Remote)
// 4. First knowledge base's SummaryModelID
func (s *sessionService) selectChatModelID(
	ctx context.Context,
	session *types.Session,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
) (string, error) {
	// If no knowledge base IDs but have knowledge IDs, derive KB IDs from knowledge IDs (include shared KB files)
	if len(knowledgeBaseIDs) == 0 && len(knowledgeIDs) > 0 {
		tenantID := types.MustTenantIDFromContext(ctx)
		knowledgeList, err := s.knowledgeService.GetKnowledgeBatchWithSharedAccess(ctx, tenantID, knowledgeIDs)
		if err != nil {
			logger.Warnf(ctx, "Failed to get knowledge batch for model selection: %v", err)
		} else {
			// Collect unique KB IDs from knowledge items
			kbIDSet := make(map[string]bool)
			for _, k := range knowledgeList {
				if k != nil && k.KnowledgeBaseID != "" {
					kbIDSet[k.KnowledgeBaseID] = true
				}
			}
			for kbID := range kbIDSet {
				knowledgeBaseIDs = append(knowledgeBaseIDs, kbID)
			}
			logger.Infof(ctx, "Derived %d knowledge base IDs from %d knowledge IDs for model selection",
				len(knowledgeBaseIDs), len(knowledgeIDs))
		}
	}
	// Check knowledge bases for models
	if len(knowledgeBaseIDs) > 0 {
		// Try to find a knowledge base with Remote model
		for _, kbID := range knowledgeBaseIDs {
			kb, err := s.knowledgeBaseService.GetKnowledgeBaseByID(ctx, kbID)
			if err != nil {
				logger.Warnf(ctx, "Failed to get knowledge base: %v", err)
				continue
			}
			if kb != nil && kb.SummaryModelID != "" {
				model, err := s.modelService.GetModelByID(ctx, kb.SummaryModelID)
				if err == nil && model != nil && model.Source == types.ModelSourceRemote {
					logger.Info(ctx, "Using Remote summary model from knowledge base")
					return kb.SummaryModelID, nil
				}
			}
		}

		// If no Remote model found, use first knowledge base's model
		kb, err := s.knowledgeBaseService.GetKnowledgeBaseByID(ctx, knowledgeBaseIDs[0])
		if err != nil {
			logger.Errorf(ctx, "Failed to get knowledge base for model ID: %v", err)
			return "", fmt.Errorf("failed to get knowledge base %s: %w", knowledgeBaseIDs[0], err)
		}
		if kb != nil && kb.SummaryModelID != "" {
			logger.Infof(
				ctx,
				"Using summary model from first knowledge base %s: %s",
				knowledgeBaseIDs[0],
				kb.SummaryModelID,
			)
			return kb.SummaryModelID, nil
		}
	}

	// No knowledge bases - try to find any available chat model
	models, err := s.modelService.ListModels(ctx)
	if err != nil {
		logger.Errorf(ctx, "Failed to list models: %v", err)
		return "", fmt.Errorf("failed to list models: %w", err)
	}
	for _, model := range models {
		if model != nil && model.Type == types.ModelTypeKnowledgeQA {
			logger.Infof(ctx, "Using first available KnowledgeQA model: %s", model.ID)
			return model.ID, nil
		}
	}

	logger.Error(ctx, "No chat model ID available")
	return "", fmt.Errorf("no chat model ID available: no knowledge bases configured and no available models")
}

// resolveKnowledgeBasesFromAgent resolves knowledge base IDs based on agent's KBSelectionMode.
// sessionTenantID is the tenant of the current session (caller); it is compared with
// customAgent.TenantID to detect the shared-agent scenario and avoid leaking the
// current user's personal shared KBs into the agent's retrieval scope.
//
// Returns the resolved knowledge base IDs based on the selection mode:
//   - "all": fetches all knowledge bases for the tenant
//   - "selected": uses the explicitly configured knowledge bases
//   - "none": returns empty slice
//   - default: falls back to configured knowledge bases for backward compatibility
func (s *sessionService) resolveKnowledgeBasesFromAgent(
	ctx context.Context,
	customAgent *types.CustomAgent,
	sessionTenantID uint64,
) []string {
	if customAgent == nil {
		return nil
	}

	switch customAgent.Config.KBSelectionMode {
	case "all":
		// Authoritative capability filter for the runtime path. The frontend
		// editor and @mention dropdown apply the same filter, but we don't
		// trust the client here: a stale session payload or API caller could
		// still ask us to retrieve against an incompatible KB and we'd rather
		// just drop it (and log) than feed it to tools that would no-op.
		capFilter := tools.DeriveKBFilterForAgent(customAgent.Config.AgentMode, customAgent.Config.AllowedTools)
		accept := func(kb *types.KnowledgeBase) bool {
			if kb == nil {
				return false
			}
			if capFilter.IsEmpty() {
				return true
			}
			return tools.KBSatisfiesAgentRequirements(kb.Capabilities(), customAgent.Config.AgentMode, customAgent.Config.AllowedTools)
		}

		// Get own knowledge bases (uses ctx TenantID = agent's tenant)
		allKBs, err := s.knowledgeBaseService.ListKnowledgeBases(ctx)
		if err != nil {
			logger.Warnf(ctx, "Failed to list all knowledge bases: %v", err)
		}
		kbIDSet := make(map[string]bool)
		kbIDs := make([]string, 0, len(allKBs))
		ownSkipped := 0
		for _, kb := range allKBs {
			if !accept(kb) {
				ownSkipped++
				continue
			}
			kbIDs = append(kbIDs, kb.ID)
			kbIDSet[kb.ID] = true
		}

		if ownSkipped > 0 {
			logger.Infof(ctx,
				"KBSelectionMode=all: tool-capability filter removed %d KBs (agent=%s, tools=%v)",
				ownSkipped, customAgent.ID, customAgent.Config.AllowedTools)
		}
		logger.Infof(ctx, "KBSelectionMode=all: loaded %d knowledge bases", len(kbIDs))
		return kbIDs
	case "selected":
		logger.Infof(ctx, "KBSelectionMode=selected: using %d configured knowledge bases", len(customAgent.Config.KnowledgeBases))
		return customAgent.Config.KnowledgeBases
	case "none":
		logger.Infof(ctx, "KBSelectionMode=none: no knowledge bases configured")
		return nil
	default:
		// Default to "selected" behavior for backward compatibility
		if len(customAgent.Config.KnowledgeBases) > 0 {
			logger.Infof(ctx, "KBSelectionMode not set: using %d configured knowledge bases", len(customAgent.Config.KnowledgeBases))
		}
		return customAgent.Config.KnowledgeBases
	}
}

// buildSearchTargets computes the unified search targets from knowledgeBaseIDs and knowledgeIDs.
// tenantID is the retrieval scope: session.TenantID or effective tenant from shared agent (set by handler).
// This is called once at the request entry point to avoid repeated queries later in the pipeline.
// Logic:
//   - For each knowledgeBaseID: resolve actual TenantID (own, org-shared, or in retrieval-tenant scope for shared agent)
//   - For each knowledgeID: find its knowledgeBaseID; if the KB is already in the list, skip; otherwise add SearchTargetTypeKnowledge
func (s *sessionService) buildSearchTargets(
	ctx context.Context,
	tenantID uint64,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
	tagScopes []types.TagScope,
) (types.SearchTargets, error) {
	caller := types.CallerFromContext(ctx)
	var targets types.SearchTargets
	tagIDsByKB := mergeTagScopesByKB(tagScopes)

	// Build a map from KB ID to TenantID for all KBs we need to process
	kbTenantMap := make(map[string]uint64)

	// Track which KBs are fully searched
	fullKBSet := make(map[string]bool)

	// First pass: batch-fetch KBs, then resolve tenant per ID (tenant scope already set by caller)
	permissions := kbReadPermissions(ctx)
	kbIDsToFetch := append([]string(nil), knowledgeBaseIDs...)
	for kbID := range tagIDsByKB {
		kbIDsToFetch = append(kbIDsToFetch, kbID)
	}
	kbIDsToFetch = uniqueNonEmptyStrings(kbIDsToFetch)

	kbByID := make(map[string]*types.KnowledgeBase)
	if len(kbIDsToFetch) > 0 {
		kbs, kbFetchErr := s.knowledgeBaseService.GetKnowledgeBasesByIDsOnly(ctx, kbIDsToFetch)
		if kbFetchErr != nil {
			logger.Warnf(ctx, "Failed to fetch knowledge bases for search targets: %v", kbFetchErr)
		}
		for _, kb := range kbs {
			if kb != nil {
				kbByID[kb.ID] = kb
			}
		}
	}
	resolveKBTenant := func(kbID string) uint64 {
		if kbTenantMap[kbID] != 0 {
			return kbTenantMap[kbID]
		}
		kb := kbByID[kbID]
		kbTenantMap[kbID] = caller.TenantID
		if kb != nil {
			kbTenantMap[kbID] = 0
			if allowed, err := permissions.Check(kbID, kb.TenantID); err == nil && allowed {
				kbTenantMap[kbID] = kb.TenantID
			}
		}
		return kbTenantMap[kbID]
	}

	if len(knowledgeBaseIDs) > 0 {
		for _, kbID := range knowledgeBaseIDs {
			fullKBSet[kbID] = true
			kbTenant := resolveKBTenant(kbID)
			if kbTenant == 0 {
				continue
			}
			if len(tagIDsByKB[kbID]) > 0 {
				continue
			}
			targets = append(targets, &types.SearchTarget{
				Type:            types.SearchTargetTypeKnowledgeBase,
				KnowledgeBaseID: kbID,
				TenantID:        kbTenant,
			})
		}
	}

	kbToKnowledgeIDs := make(map[string][]string)

	// Process individual knowledge IDs (include shared KB files the user has access to)
	if len(knowledgeIDs) > 0 {
		knowledgeList, err := s.knowledgeService.GetKnowledgeBatchWithSharedAccess(ctx, tenantID, knowledgeIDs)
		if err != nil {
			logger.Warnf(ctx, "Failed to get knowledge batch for search targets: %v", err)
			return targets, nil // Return what we have, don't fail
		}

		// Group knowledge IDs by their KB, excluding those already covered by full KB search
		// Also track KB tenant IDs from knowledge items
		for _, k := range knowledgeList {
			if k == nil || k.KnowledgeBaseID == "" {
				continue
			}
			// Track KB -> TenantID mapping from knowledge items
			if kbTenantMap[k.KnowledgeBaseID] == 0 {
				kbTenantMap[k.KnowledgeBaseID] = k.TenantID
			}
			// Skip if this KB is already fully searched without a tag scope.
			if fullKBSet[k.KnowledgeBaseID] && len(tagIDsByKB[k.KnowledgeBaseID]) == 0 {
				continue
			}
			kbToKnowledgeIDs[k.KnowledgeBaseID] = append(kbToKnowledgeIDs[k.KnowledgeBaseID], k.ID)
		}

		// Create SearchTargetTypeKnowledge targets for each KB with specific files
		for kbID, kidList := range kbToKnowledgeIDs {
			if len(tagIDsByKB[kbID]) > 0 {
				continue
			}
			kbTenant := kbTenantMap[kbID]
			if kbTenant == 0 {
				kbTenant = tenantID // fallback
			}
			targets = append(targets, &types.SearchTarget{
				Type:                    types.SearchTargetTypeKnowledge,
				KnowledgeBaseID:         kbID,
				TenantID:                kbTenant,
				KnowledgeIDs:            kidList,
				DisableRecallThresholds: true,
			})
		}
	}

	for kbID, tagIDs := range tagIDsByKB {
		if kbID == "" || len(tagIDs) == 0 {
			continue
		}
		kbTenant := resolveKBTenant(kbID)
		if kbTenant == 0 {
			continue
		}
		kb := kbByID[kbID]
		explicitKnowledgeIDs := uniqueNonEmptyStrings(kbToKnowledgeIDs[kbID])

		useDocumentTagResolution := kb == nil || kb.Type != types.KnowledgeBaseTypeFAQ
		if kb == nil {
			logger.Warnf(ctx, "Knowledge base metadata missing for tag scope, kb_id=%s, using document tag resolution", kbID)
		}
		if useDocumentTagResolution {
			tagKnowledgeIDs, err := s.knowledgeService.ListKnowledgeIDsByTagIDs(ctx, kbTenant, kbID, tagIDs)
			if err != nil {
				return nil, fmt.Errorf("resolve knowledge IDs for tag scope kb_id=%s: %w", kbID, err)
			}
			if len(explicitKnowledgeIDs) > 0 {
				tagKnowledgeIDs = intersectStrings(tagKnowledgeIDs, explicitKnowledgeIDs)
			}
			tagKnowledgeIDs = uniqueNonEmptyStrings(tagKnowledgeIDs)
			if len(tagKnowledgeIDs) == 0 {
				continue
			}
			targets = append(targets, &types.SearchTarget{
				Type:                    types.SearchTargetTypeKnowledge,
				KnowledgeBaseID:         kbID,
				TenantID:                kbTenant,
				KnowledgeIDs:            tagKnowledgeIDs,
				ScopeTagIDs:             append([]string(nil), tagIDs...),
				DisableRecallThresholds: true,
			})
			continue
		}

		target := &types.SearchTarget{
			Type:                    types.SearchTargetTypeKnowledgeBase,
			KnowledgeBaseID:         kbID,
			TenantID:                kbTenant,
			TagIDs:                  append([]string(nil), tagIDs...),
			ScopeTagIDs:             append([]string(nil), tagIDs...),
			DisableRecallThresholds: true,
		}
		if len(explicitKnowledgeIDs) > 0 {
			target.Type = types.SearchTargetTypeKnowledge
			target.KnowledgeIDs = explicitKnowledgeIDs
			target.DisableRecallThresholds = true
		}
		targets = append(targets, target)
	}

	logger.Infof(ctx, "Built %d search targets: %d full KB, %d partial/tag KB, kbTenantMap=%v",
		len(targets), len(knowledgeBaseIDs), len(targets)-len(knowledgeBaseIDs), kbTenantMap)

	return targets, nil
}

func mergeTagScopesByKB(scopes []types.TagScope) map[string][]string {
	byKB := make(map[string][]string)
	seen := make(map[string]map[string]bool)
	for _, scope := range scopes {
		if scope.KnowledgeBaseID == "" {
			continue
		}
		if seen[scope.KnowledgeBaseID] == nil {
			seen[scope.KnowledgeBaseID] = make(map[string]bool)
		}
		for _, tagID := range scope.TagIDs {
			if tagID == "" || seen[scope.KnowledgeBaseID][tagID] {
				continue
			}
			seen[scope.KnowledgeBaseID][tagID] = true
			byKB[scope.KnowledgeBaseID] = append(byKB[scope.KnowledgeBaseID], tagID)
		}
	}
	return byKB
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func intersectStrings(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	rightSet := make(map[string]bool, len(right))
	for _, value := range right {
		rightSet[value] = true
	}
	out := make([]string, 0)
	for _, value := range left {
		if rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}
