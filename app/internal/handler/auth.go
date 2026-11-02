package handler

import (
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gaze/internal/application/service"
	"gaze/internal/errors"
	"gaze/internal/handler/dto"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
	secutils "gaze/internal/utils"
)

// AuthHandler implements the personal edition's authentication surface:
// login, logout, token refresh/validation and password change for the single
// owner account.
type AuthHandler struct {
	userService   interfaces.UserService
	tenantService interfaces.TenantService
}

func NewAuthHandler(userService interfaces.UserService, tenantService interfaces.TenantService) *AuthHandler {
	return &AuthHandler{userService: userService, tenantService: tenantService}
}

// Login godoc
// @Summary      用户登录
// @Description  用户登录并获取访问令牌
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      types.LoginRequest  true  "登录请求参数"
// @Success      200      {object}  types.LoginResponse
// @Failure      401      {object}  errors.AppError  "认证失败"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start user login")

	var req types.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse login request parameters", err)
		appErr := errors.NewValidationError("Invalid login parameters").WithDetails(err.Error())
		c.Error(appErr)
		return
	}
	email := secutils.SanitizeForLog(req.Email)

	if req.Email == "" || req.Password == "" {
		logger.Error(ctx, "Missing required login fields")
		appErr := errors.NewValidationError("Email and password are required")
		c.Error(appErr)
		return
	}

	response, err := h.userService.Login(ctx, &req)
	if err != nil {
		logger.Errorf(ctx, "Failed to login user: %v", err)
		appErr := errors.NewUnauthorizedError("Login failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	if !response.Success {
		logger.Warnf(ctx, "Login failed: %s", response.Message)
		c.JSON(http.StatusUnauthorized, dto.NewAuthLoginResponse(response))
		return
	}

	logger.Infof(ctx, "User logged in successfully, email: %s", email)
	c.JSON(http.StatusOK, dto.NewAuthLoginResponse(response))
}

// Logout godoc
// @Summary      用户登出
// @Description  撤销当前访问令牌并登出
// @Tags         认证
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "登出成功"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start user logout")

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		logger.Error(ctx, "Missing Authorization header")
		appErr := errors.NewValidationError("Authorization header is required")
		c.Error(appErr)
		return
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		logger.Error(ctx, "Invalid Authorization header format")
		appErr := errors.NewValidationError("Invalid Authorization header format")
		c.Error(appErr)
		return
	}

	token := tokenParts[1]

	// Revoke every outstanding session for this user so refresh tokens
	// cannot keep working after logout.
	err := h.userService.Logout(ctx, token)
	if err != nil {
		logger.Errorf(ctx, "Failed to revoke token: %v", err)
		appErr := errors.NewInternalServerError("Logout failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	logger.Info(ctx, "User logged out successfully")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful",
	})
}

// RefreshToken godoc
// @Summary      刷新令牌
// @Description  使用刷新令牌获取新的访问令牌
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      object{refreshToken=string}  true  "刷新令牌"
// @Success      200      {object}  map[string]interface{}       "新令牌"
// @Failure      401      {object}  errors.AppError              "令牌无效"
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start token refresh")

	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse refresh token request", err)
		appErr := errors.NewValidationError("Invalid refresh token request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	accessToken, newRefreshToken, err := h.userService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		logger.Errorf(ctx, "Failed to refresh token: %v", err)
		appErr := errors.NewUnauthorizedError("Token refresh failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	logger.Info(ctx, "Token refreshed successfully")
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Token refreshed successfully",
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// GetCurrentUser godoc
// @Summary      获取当前用户信息
// @Description  获取当前登录用户的详细信息
// @Tags         认证
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "用户信息"
// @Failure      401  {object}  errors.AppError         "未授权"
// @Security     Bearer
// @Router       /auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil {
		logger.Errorf(ctx, "Failed to get current user: %v", err)
		appErr := errors.NewUnauthorizedError("Failed to get user information").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	userInfo := user.ToUserInfo()
	var tenant *types.Tenant
	if user.TenantID != 0 && h.tenantService != nil {
		if t, terr := h.tenantService.GetTenantByID(ctx, user.TenantID); terr == nil {
			tenant = t
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user":   userInfo,
			"tenant": tenant,
		},
	})
}

// ChangePassword godoc
// @Summary      修改密码
// @Description  修改当前用户的登录密码。新密码须满足 8–32 位且同时包含字母与数字；开启复杂密码后还需包含大小写与特殊字符。成功后所有会话被撤销，需重新登录。
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      object{old_password=string,new_password=string}  true  "密码修改请求"
// @Success      200      {object}  map[string]interface{}                           "修改成功"
// @Failure      400      {object}  errors.AppError                                  "请求参数错误"
// @Security     Bearer
// @Router       /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start password change")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse password change request", err)
		appErr := errors.NewValidationError("Invalid password change request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil {
		logger.Errorf(ctx, "Failed to get current user: %v", err)
		appErr := errors.NewUnauthorizedError("Failed to get user information").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	err = h.userService.ChangePassword(ctx, user.ID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case service.IsPasswordPolicyError(err):
			appErr := errors.NewValidationError("Password policy violation").
				WithDetails(service.DetailPasswordPolicy)
			_ = c.Error(appErr)
			return
		case stderrors.Is(err, service.ErrInvalidOldPassword):
			appErr := errors.NewBadRequestError("Current password is incorrect").
				WithDetails(service.DetailInvalidOldPassword)
			c.Error(appErr)
			return
		case stderrors.Is(err, service.ErrSamePassword):
			appErr := errors.NewValidationError("New password must differ from current password").
				WithDetails(service.DetailSamePassword)
			c.Error(appErr)
			return
		default:
			logger.Errorf(ctx, "Failed to change password: %v", err)
			appErr := errors.NewBadRequestError("Password change failed").WithDetails(err.Error())
			c.Error(appErr)
			return
		}
	}

	logger.Infof(ctx, "Password changed successfully for user: %s", user.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed successfully",
	})
}

// ValidateToken godoc
// @Summary      验证令牌
// @Description  验证访问令牌是否有效
// @Tags         认证
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "令牌有效"
// @Failure      401  {object}  errors.AppError         "令牌无效"
// @Security     Bearer
// @Router       /auth/validate [get]
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start token validation")

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		logger.Error(ctx, "Missing Authorization header")
		appErr := errors.NewValidationError("Authorization header is required")
		c.Error(appErr)
		return
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		logger.Error(ctx, "Invalid Authorization header format")
		appErr := errors.NewValidationError("Invalid Authorization header format")
		c.Error(appErr)
		return
	}

	token := tokenParts[1]

	user, _, err := h.userService.ValidateToken(ctx, token)
	if err != nil {
		logger.Errorf(ctx, "Failed to validate token: %v", err)
		appErr := errors.NewUnauthorizedError("Token validation failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	logger.Infof(ctx, "Token validated successfully for user: %s", user.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token is valid",
		"user":    user.ToUserInfo(),
	})
}
