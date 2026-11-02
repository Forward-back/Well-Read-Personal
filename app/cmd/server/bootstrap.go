// Bootstrap-time hooks that run after the DI container is built but
// before the HTTP server starts listening. Owner creation is fail-fast:
// without an owner account nobody can log in, so a misconfigured deploy
// aborts instead of serving a locked instance. The legacy API-key hash
// backfill stays best-effort.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/dig"

	apprepo "gaze/internal/application/repository"
	"gaze/internal/application/service"
	"gaze/internal/logger"
	"gaze/internal/types"
	"gaze/internal/types/interfaces"
)

const (
	ownerEmailEnv    = "OWNER_EMAIL"
	ownerPasswordEnv = "OWNER_PASSWORD"
)

// runStartupBootstrap creates the single personal owner account from the
// environment when the database has no account yet. It never creates a
// second account: an existing installation keeps its current owner.
func runStartupBootstrap(c *dig.Container) error {
	ctx := context.Background()

	// Legacy hash repair for pre-existing rows. Kept until the personal
	// API-key schema replaces the enterprise key table.
	if err := c.Invoke(func(apiKeySvc interfaces.TenantAPIKeyService) {
		if n, err := apiKeySvc.BackfillMissingKeyHashes(ctx); err != nil {
			logger.Warnf(ctx, "[bootstrap] tenant api key hash backfill failed: %v", err)
		} else if n > 0 {
			logger.Infof(ctx, "[bootstrap] backfilled %d legacy tenant api key hash(es)", n)
		}
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to resolve TenantAPIKeyService: %v", err)
	}

	email := strings.TrimSpace(os.Getenv(ownerEmailEnv))
	password := os.Getenv(ownerPasswordEnv)
	if email == "" || password == "" {
		return fmt.Errorf("%s and %s must be set to create the personal owner account",
			ownerEmailEnv, ownerPasswordEnv)
	}
	if err := service.ValidatePasswordPolicy(password, false); err != nil {
		return fmt.Errorf("invalid %s: %w", ownerPasswordEnv, err)
	}

	var ownerErr error
	if err := c.Invoke(func(userSvc interfaces.UserService) {
		ownerErr = ensureOwner(ctx, userSvc, email, password)
	}); err != nil {
		return fmt.Errorf("resolve UserService: %w", err)
	}
	return ownerErr
}

func ensureOwner(ctx context.Context, userSvc interfaces.UserService, email, password string) error {
	user, err := userSvc.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, apprepo.ErrUserNotFound) {
		return fmt.Errorf("owner lookup failed: %w", err)
	}
	if user != nil {
		if !user.IsSystemAdmin {
			user.IsSystemAdmin = true
			if err := userSvc.UpdateUser(ctx, user); err != nil {
				return fmt.Errorf("promote owner %s: %w", user.ID, err)
			}
		}
		logger.Infof(ctx, "[bootstrap] owner account ready: %s", email)
		return nil
	}

	existing, err := userSvc.ListUsers(ctx, 0, 1)
	if err != nil {
		return fmt.Errorf("cannot check existing accounts: %w", err)
	}
	if len(existing) > 0 {
		return fmt.Errorf(
			"%s=%s not found but the database already has an account; refusing to create a second owner",
			ownerEmailEnv, email)
	}

	username := strings.TrimSpace(strings.SplitN(email, "@", 2)[0])
	if username == "" {
		username = "owner"
	}
	if len(username) > 50 {
		username = username[:50]
	}

	created, err := userSvc.Register(ctx, &types.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
	})
	if err != nil {
		return fmt.Errorf("create owner: %w", err)
	}
	created.IsSystemAdmin = true
	if err := userSvc.UpdateUser(ctx, created); err != nil {
		return fmt.Errorf("owner created but admin flag update failed: %w", err)
	}
	logger.Infof(ctx, "[bootstrap] created personal owner %s (%s)", username, email)
	return nil
}
