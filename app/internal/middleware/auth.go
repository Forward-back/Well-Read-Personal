package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// Endpoints that are reachable without an authenticated owner session.
var noAuthAPI = map[string][]string{
	"/health":                 {"GET"},
	"/api/v1/auth/login":      {"POST"},
	"/api/v1/auth/refresh":    {"POST"},
	"/api/v1/files/presigned": {"GET", "HEAD"},
}

func isNoAuthAPI(path string, method string) bool {
	methods, ok := noAuthAPI[path]
	return ok && slices.Contains(methods, method)
}

// Auth authenticates a request as either the owner's browser session (Bearer
// JWT) or the owner's API key. There is no workspace, membership or role
// resolution: every authenticated request belongs to the single owner.
func Auth(
	userService interfaces.UserService,
	apiKeyService interfaces.TenantAPIKeyService,
	tenantService interfaces.TenantService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if isNoAuthAPI(c.Request.URL.Path, c.Request.Method) {
			c.Next()
			return
		}

		bearerPresented := false
		if token, ok := bearerToken(c); ok {
			bearerPresented = true
			user, _, err := userService.ValidateToken(c.Request.Context(), token)
			if err == nil && user != nil {
				attachOwnerSession(c, user, loadOwnerTenant(c.Request.Context(), tenantService, user), nil)
				c.Next()
				return
			}
			logger.Warnf(c.Request.Context(), "[auth] bearer token rejected: %v", err)
		}

		if apiKey := strings.TrimSpace(c.GetHeader("X-API-Key")); apiKey != "" {
			if apiKeyService == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: API key service is not configured"})
				c.Abort()
				return
			}
			key, err := apiKeyService.AuthenticateAPIKey(c.Request.Context(), apiKey)
			if err != nil || key == nil || key.IsPlatform() {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid API key"})
				c.Abort()
				return
			}
			tenantID := key.TenantIDValue()
			if tenantID == 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid API key scope"})
				c.Abort()
				return
			}
			user, err := userService.GetUserByTenantID(c.Request.Context(), tenantID)
			if err != nil || user == nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid API key"})
				c.Abort()
				return
			}
			attachOwnerSession(c, user, loadOwnerTenant(c.Request.Context(), tenantService, user), &types.TenantAPIKeyScope{
				KeyID:            key.ID,
				ScopeType:        key.ScopeType,
				FullAccess:       key.FullAccess,
				KnowledgeBaseIDs: key.KnowledgeBaseIDs,
				Capabilities:     key.Capabilities,
			})
			c.Next()
			return
		}

		if bearerPresented {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid or expired token"})
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing authentication"})
		}
		c.Abort()
	}
}

func bearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(authHeader, "Bearer "), true
}

func loadOwnerTenant(ctx context.Context, tenantService interfaces.TenantService, user *types.User) *types.Tenant {
	if tenantService == nil || user == nil || user.TenantID == 0 {
		return nil
	}
	tenant, err := tenantService.GetTenantByID(ctx, user.TenantID)
	if err != nil {
		logger.Warnf(ctx, "[auth] load tenant %d for user %s failed: %v", user.TenantID, user.ID, err)
		return nil
	}
	return tenant
}

func attachOwnerSession(c *gin.Context, user *types.User, tenant *types.Tenant, apiKeyScope *types.TenantAPIKeyScope) {
	applyAuthSession(c, authSession{
		User:        user,
		Principal:   types.Principal{Type: types.PrincipalWebUser, ID: user.ID},
		TenantID:    user.TenantID,
		Tenant:      tenant,
		Role:        types.TenantRoleOwner,
		SystemAdmin: user.IsSystemAdmin,
		APIKeyScope: apiKeyScope,
	})
}
