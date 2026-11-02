package router

import (
	"net/http"
	"path"
	"strings"

	"gaze/internal/middleware"
	"gaze/internal/types"
	"github.com/gin-gonic/gin"
)

// apiKeyGuards records which routes the personal MCP API key may call. The
// key is read-only, but an explicit allow-list keeps the surface declarative
// and denies every undeclared route by default.
//
// Routes opt in via apiKeyGroup / apiKeyRoute, declaring a policy built from
// the constructors below. JWT sessions ignore the gate entirely.
type apiKeyGuards struct {
	apiKeyAuthorizer *middleware.APIKeyRouteAuthorizer
}

func newAPIKeyGuards() *apiKeyGuards {
	return &apiKeyGuards{apiKeyAuthorizer: middleware.NewAPIKeyRouteAuthorizer()}
}

func apiKeyAny() middleware.APIKeyRoutePolicy {
	return middleware.APIKeyRoutePolicy{}
}

func apiKeyFullAccess() middleware.APIKeyRoutePolicy {
	return middleware.APIKeyRoutePolicy{RequireFullAccess: true}
}

// apiKeyRetrieve grants read/search access to knowledge-base data.
func apiKeyRetrieve(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityRetrieve)
}

// apiKeyChat layers the "chat" capability on top of a base policy so keys can
// use the conversation flow (sessions, agent listing).
func apiKeyChat(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityChat)
}

// apiKeyReadAgents lets external integrations resolve agent IDs without chat
// or authoring authority.
func apiKeyReadAgents(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityReadAgents)
}

// apiKeyIngest lets a scoped key write content into its allowed knowledge
// bases (documents, chunks, FAQ, tags, gaze).
func apiKeyIngest(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityIngest)
}

// apiKeyManageKnowledgeBases lets a scoped key manage the KB lifecycle
// (create/copy/duplicate/update/delete + config).
func apiKeyManageKnowledgeBases(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageKnowledgeBases)
}

// apiKeyMessageHistory lets an explicitly granted key search or inspect chat
// history without being promoted to full owner access.
func apiKeyMessageHistory(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityMessageHistory)
}

func apiKeyManageModels(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageModels)
}

func apiKeyManageDataSources(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageDataSources)
}

func apiKeyManageVectorStores(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageVectorStores)
}

func apiKeyManageStorageBackends(base middleware.APIKeyRoutePolicy) middleware.APIKeyRoutePolicy {
	return base.WithCapability(types.APIKeyCapabilityManageStorageBackends)
}

// apiKeyRouteGroup wraps a *gin.RouterGroup so route registration also records
// the route's API-key policy into the authorizer. Use g.apiKeyGroup(grp,
// policy) then register the API-key-accessible routes through it; register
// API-key-denied routes on the raw *gin.RouterGroup so they stay undeclared
// (default-deny). Per-route overrides use With().
type apiKeyRouteGroup struct {
	g      *apiKeyGuards
	grp    *gin.RouterGroup
	policy middleware.APIKeyRoutePolicy
}

// apiKeyGroup returns a wrapper that declares `policy` for every route
// registered through it (unless overridden via With).
func (g *apiKeyGuards) apiKeyGroup(grp *gin.RouterGroup, policy middleware.APIKeyRoutePolicy) *apiKeyRouteGroup {
	return &apiKeyRouteGroup{g: g, grp: grp, policy: policy}
}

// With returns a sibling wrapper on the same gin group but with a different
// policy, for the odd route that differs from its group default.
func (a *apiKeyRouteGroup) With(policy middleware.APIKeyRoutePolicy) *apiKeyRouteGroup {
	return &apiKeyRouteGroup{g: a.g, grp: a.grp, policy: policy}
}

func (a *apiKeyRouteGroup) handle(method, rel string, handlers ...gin.HandlerFunc) gin.IRoutes {
	full := path.Join(a.grp.BasePath(), rel)
	a.g.apiKeyAuthorizer.Register(method, full, a.policy)
	return a.grp.Handle(method, rel, handlers...)
}

func (a *apiKeyRouteGroup) GET(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodGet, rel, h...)
}

func (a *apiKeyRouteGroup) POST(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPost, rel, h...)
}

func (a *apiKeyRouteGroup) PUT(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPut, rel, h...)
}

func (a *apiKeyRouteGroup) PATCH(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodPatch, rel, h...)
}

func (a *apiKeyRouteGroup) DELETE(rel string, h ...gin.HandlerFunc) gin.IRoutes {
	return a.handle(http.MethodDelete, rel, h...)
}

// apiKeyRoute declares a single API-key-accessible route directly on a gin
// group (for routes registered outside an apiKeyGroup, e.g. top-level r.POST).
func (g *apiKeyGuards) apiKeyRoute(
	grp *gin.RouterGroup, method, rel string, policy middleware.APIKeyRoutePolicy, handlers ...gin.HandlerFunc,
) gin.IRoutes {
	full := path.Join(grp.BasePath(), rel)
	g.apiKeyAuthorizer.Register(method, full, policy)
	return grp.Handle(method, rel, handlers...)
}

// assertAPIKeyPoliciesMatchRoutes verifies every declared API-key policy
// resolves to a real registered route. gin's c.FullPath() must match the
// authorizer key verbatim or the gate silently 403s the route for API keys;
// panicking here turns that latent misconfiguration into a startup failure.
func (g *apiKeyGuards) assertAPIKeyPoliciesMatchRoutes(engine *gin.Engine) {
	registered := map[string]struct{}{}
	for _, ri := range engine.Routes() {
		// Authorizer keys are stored normalized (trailing slash trimmed), so
		// normalize gin's reported path the same way. Otherwise a route
		// registered with a "/" rel (gin path ".../evaluation/") would look
		// missing against the normalized key (".../evaluation") even though
		// the gate — which also normalizes c.FullPath() — matches it fine.
		p := ri.Path
		if len(p) > 1 {
			p = strings.TrimRight(p, "/")
		}
		registered[ri.Method+" "+p] = struct{}{}
	}
	var missing []string
	for method, paths := range g.apiKeyAuthorizer.RegisteredRoutes() {
		for _, p := range paths {
			if _, ok := registered[method+" "+p]; !ok {
				missing = append(missing, method+" "+p)
			}
		}
	}
	if len(missing) > 0 {
		panic("api-key policy declared for non-existent route(s): " + strings.Join(missing, ", "))
	}
}
