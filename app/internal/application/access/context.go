package access

import (
	"context"

	"gaze/internal/types"
)

// Context grants contain values, not mutable KB objects. Deriving a context
// copies the grant slice so parallel search branches cannot widen each other.
type kbGrant struct {
	caller   types.Caller
	kbID     string
	tenantID uint64
	task     bool
}

// HasKBGrant reports whether the caller may use a KB at the required level.
// The personal edition has one owner per deployment, so tenant ownership is
// the business boundary; explicit grants still serve trusted task workers.
func HasKBGrant(ctx context.Context, kbID string, tenantID uint64) bool {
	if types.AuthorizeTenantAPIKeyKnowledgeBases(ctx, kbID) != nil {
		return false
	}
	caller := types.CallerFromContext(ctx)
	if kbID == "" || tenantID == 0 {
		return false
	}
	if caller.TenantID == tenantID {
		return true
	}
	grants, _ := ctx.Value(types.KBGrantsContextKey).([]kbGrant)
	for _, grant := range grants {
		if (caller.TenantID != 0 || grant.task) && grant.caller == caller && grant.kbID == kbID &&
			grant.tenantID == tenantID {
			return true
		}
	}
	return false
}

// KBPermissions applies the single-owner KB rule for one service operation.
type KBPermissions struct {
	ctx    context.Context
	caller types.Caller
}

// NewKBPermissions resolves reads for one caller and operation.
func NewKBPermissions(ctx context.Context) *KBPermissions {
	return &KBPermissions{ctx: ctx, caller: types.CallerFromContext(ctx)}
}

// CheckKB reports whether this caller may use the KB at the required level.
func (p *KBPermissions) CheckKB(kb *types.KnowledgeBase) (bool, error) {
	if kb == nil || kb.ID == "" || kb.TenantID == 0 {
		return false, nil
	}
	if err := types.AuthorizeTenantAPIKeyKnowledgeBases(p.ctx, kb.ID); err != nil {
		return false, err
	}
	return HasKBGrant(p.ctx, kb.ID, kb.TenantID), nil
}

// Check is the ID-only form for retrieval callers.
func (p *KBPermissions) Check(kbID string, ownerTenantID uint64) (bool, error) {
	if kbID == "" || ownerTenantID == 0 {
		return false, nil
	}
	if err := types.AuthorizeTenantAPIKeyKnowledgeBases(p.ctx, kbID); err != nil {
		return false, err
	}
	return HasKBGrant(p.ctx, kbID, ownerTenantID), nil
}
