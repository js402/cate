package serverops

import (
	"context"
	"fmt"
	"time"

	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libauth"
)

const DefaultServerGroup = "server"
const DefaultDefaultServiceGroup = "admin_panel"
const DefaultAdminUser = "admin@admin.com"

// CheckResourceAuthorization checks if the user has the required permission for a given resource.
func CheckResourceAuthorization(ctx context.Context, resource string, requiredPermission store.Permission) error {
	if instance := GetManagerInstance(); instance == nil {
		return fmt.Errorf("BUG: Service Manager was not initialized")
	}
	if instance := GetManagerInstance(); instance != nil && instance.IsSecurityEnabled(DefaultServerGroup) {
		// Get the access entries for the user from the token
		accessList, err := libauth.GetClaims[store.AccessList](ctx, instance.GetSecret())
		if err != nil {
			return fmt.Errorf("failed to get user claims: %w", err)
		}

		// Check if any of the user's access entries allow the required permission on the resource
		authorized, err := accessList.RequireAuthorisation(resource, int(requiredPermission))
		if err != nil {
			return fmt.Errorf("error during authorization check: %w", err)
		}

		if !authorized {
			return fmt.Errorf("unauthorized: user lacks permission %v for resource %s", requiredPermission, resource)
		}
	}
	return nil

}

func CheckServiceAuthorization[T ServiceMeta](ctx context.Context, s T, permission store.Permission) error {
	instance := GetManagerInstance()
	if instance == nil {
		return fmt.Errorf("BUG: Service Manager was not initialized")
	}
	if !instance.IsSecurityEnabled(DefaultServerGroup) {
		return nil
	}

	tryAuth := []string{
		s.GetServiceName(),
		s.GetServiceGroup(),
		DefaultServerGroup,
	}
	var authorized bool
	var err error
	for _, resource := range tryAuth {
		authorized, err = libauth.CheckAuthorisation[store.AccessList](ctx, instance.GetSecret(), resource, int(permission))
		if err != nil {
			return err
		}
		if authorized {
			break
		}
	}
	if authorized {
		return nil
	}
	return fmt.Errorf("service %s is not authorized: %w", s.GetServiceName(), libauth.ErrNotAuthorized)
}

func CreateAuthToken(subject string, permissions store.AccessList) (string, time.Time, error) {
	instance := GetManagerInstance()
	if instance == nil {
		return "", time.Time{}, fmt.Errorf("BUG: Service Manager was not initialized")
	}

	cfg := libauth.CreateTokenArgs{
		JWTSecret: instance.GetSecret(),
		JWTExpiry: instance.GetTokenExpiry(),
	}
	// Delegate token creation to libauth.
	token, expiresAt, err := libauth.CreateToken[store.AccessList](cfg, subject, permissions)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create token: %w", err)
	}
	return token, expiresAt, nil
}

func RefreshToken(ctx context.Context) (string, bool, time.Time, error) {
	tokenString, ok := ctx.Value(libauth.ContextTokenKey).(string)
	if !ok {
		return "", false, time.Time{}, fmt.Errorf("BUG: token not found in context")
	}
	gracePeriod := time.Minute * 20
	return RefreshPlainToken(ctx, tokenString, &gracePeriod)
}

func RefreshPlainToken(ctx context.Context, token string, withGracePeriod *time.Duration) (string, bool, time.Time, error) {
	instance := GetManagerInstance()
	if instance == nil {
		return "", false, time.Time{}, fmt.Errorf("BUG: Service Manager was not initialized")
	}
	if !instance.IsSecurityEnabled(DefaultServerGroup) {
		return "", false, time.Time{}, nil
	}
	cfg := libauth.CreateTokenArgs{
		JWTSecret: instance.GetSecret(),
		JWTExpiry: instance.GetTokenExpiry(),
	}
	if withGracePeriod == nil {
		tokenString, expiresAt, err := libauth.RefreshToken[store.AccessList](cfg, token)
		if err != nil {
			return "", false, time.Time{}, fmt.Errorf("failed to refresh token: %w", err)
		}
		return tokenString, true, expiresAt, nil
	}

	tokenString, wasReplaced, expiresAt, err := libauth.RefreshTokenWithGracePeriod[store.AccessList](cfg, token, *withGracePeriod)
	if err != nil {
		return "", false, time.Time{}, fmt.Errorf("failed to refresh token: %w", err)
	}

	return tokenString, wasReplaced, expiresAt, nil
}

// GetIdentity extracts the identity from the context using the JWT secret from the ServiceManager.
func GetIdentity(ctx context.Context) (string, error) {
	manager := GetManagerInstance()
	if manager == nil {
		return "", fmt.Errorf("service manager is not initialized")
	}
	if !manager.IsSecurityEnabled(DefaultServerGroup) {
		return DefaultAdminUser, nil
	}
	jwtSecret := manager.GetSecret()
	if jwtSecret == "" {
		return "", libauth.ErrEmptyJWTSecret
	}

	return libauth.GetIdentity[store.AccessList](ctx, jwtSecret)
}
