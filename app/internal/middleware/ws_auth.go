// Package middleware: authentication for browser WebSocket upgrades.
//
// Browsers cannot attach custom headers (Authorization / X-API-Key) to a
// WebSocket handshake. The sandbox terminal mints a short-lived ticket over
// a normal authenticated POST and presents it as the ticket query parameter;
// AttachAuthenticatedUser then installs the owner session for that connection.
package middleware

import (
	"net/http"

	"gaze/internal/types"
	"github.com/gin-gonic/gin"
)

// AttachAuthenticatedUser installs the owner session for an already-resolved
// user. On success the request carries the session and true is returned; on
// failure the response has already been written and false is returned.
func AttachAuthenticatedUser(c *gin.Context, user *types.User, tenant *types.Tenant) bool {
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid or expired token"})
		c.Abort()
		return false
	}
	attachOwnerSession(c, user, tenant, nil)
	return true
}
