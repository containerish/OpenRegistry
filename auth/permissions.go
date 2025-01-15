package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) RepositoryPermissionsMiddleware() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			err, responseHandled := a.handleTokenRequest(ctx, handler)
			if err != nil {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeDenied,
					"invalid user credentials",
					echo.Map{
						"error": err.Error(),
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				a.logger.DebugWithContext(ctx).Err(registryErr).Send()
				return echoErr
			}

			if responseHandled {
				return nil
			}

			namespace, err := a.getImageNamespace(ctx)
			if err != nil {
				errMsg := common.RegistryErrorResponse(
					registry.RegistryErrorCodeUnknown,
					"invalid image namespace",
					echo.Map{
						"error": err.Error(),
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
				a.logger.DebugWithContext(ctx).Err(err).Send()
				return echoErr
			}

			repository, err := a.registryStore.GetRepositoryByNamespace(ctx.Request().Context(), namespace)
			if err == nil {
				if repository.Visibility == types.RepositoryVisibilityPublic {
					a.logger.DebugWithContext(ctx).Send()
					return handler(ctx)
				}
			}

			user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
			if !ok {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeUnauthorized,
					"access to this resource is restricted, please login or check with the repository owner",
					echo.Map{
						"error": "authentication details are missing",
					},
				)
				echoErr := ctx.JSONBlob(http.StatusForbidden, registryErr.Bytes())
				a.logger.DebugWithContext(ctx).Err(registryErr).Send()
				return echoErr
			}

			if err = a.validateUserPermissions(ctx, namespace, user, handler); err != nil {
				errMsg := common.RegistryErrorResponse(
					registry.RegistryErrorCodeUnauthorized,
					"access to this resource is restricted, please login or check with the repository owner",
					echo.Map{
						"error": err.Error(),
					},
				)
				echoErr := ctx.JSONBlob(http.StatusForbidden, errMsg.Bytes())
				a.logger.DebugWithContext(ctx).Err(errMsg).Send()
				return echoErr
			}

			a.logger.DebugWithContext(ctx).Send()
			return handler(ctx)
		}
	}
}

func (a *auth) getImageNamespace(ctx echo.Context) (string, error) {
	if ctx.Request().URL.Path == "/token" {
		scope, err := a.getScopeFromQueryParams(ctx.QueryParam("scope"))
		if err != nil {
			return "", err
		}

		return scope.Name, nil
	}
	username := ctx.Param("username")
	imageName := ctx.Param("imagename")
	return username + "/" + imageName, nil
}

func (a *auth) populateUserFromPermissionsCheck(ctx echo.Context) error {
	auth := ctx.Request().Header.Get(echo.HeaderAuthorization)
	isTokenRequest := ctx.Request().URL.Path == "/token"

	if len(auth) > len(authSchemeBasic)+1 && strings.EqualFold(auth[:len(authSchemeBasic)], authSchemeBasic) {
		user, err := a.validateBasicAuthCredentials(auth)
		if err != nil {
			return fmt.Errorf("ERR_USER_PERM_CHECK: %w", err)
		}

		ctx.Set(string(types.UserContextKey), user)
		return nil
	}

	// Check if it's an OCI request
	if !isTokenRequest {
		if _, ok := ctx.Get(string(types.UserContextKey)).(*types.User); ok {
			return nil
		}
	}

	return fmt.Errorf("invalid user credentials: %s", auth)
}

func (a *auth) handleTokenRequest(ctx echo.Context, handler echo.HandlerFunc) (error, bool) {
	if err := a.populateUserFromPermissionsCheck(ctx); err != nil {
		registryErr := common.RegistryErrorResponse(
			registry.RegistryErrorCodeUnauthorized,
			err.Error(),
			nil,
		)

		echoErr := ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
		a.logger.DebugWithContext(ctx).Err(registryErr).Send()
		return echoErr, true
	}

	if ctx.QueryParam("offline_token") == "true" {
		a.logger.DebugWithContext(ctx).Send()
		return handler(ctx), true
	}

	return nil, false
}

func (a *auth) validateUserPermissions(ctx echo.Context, ns string, user *types.User, handler echo.HandlerFunc) error {
	permissions := a.
		permissionsStore.
		GetUserPermissionsForNamespace(
			ctx.Request().Context(),
			ns,
			user.ID,
		)

	usernameFromReq := strings.Split(ns, "/")[0]
	ctx.Set(string(types.UserPermissionsContextKey), permissions)
	readOp := ctx.Request().Method == http.MethodGet || ctx.Request().Method == http.MethodHead
	permissonAllowed := permissions.IsAdmin || (readOp && permissions.Pull) || (!readOp && permissions.Push)
	isTokenRequest := ctx.Request().URL.Path == "/token"

	if permissonAllowed || user.Username == usernameFromReq || usernameFromReq == types.SystemUsernameIPFS {
		// if someone else is making the request on behalf of the org, then we set org as the underyling user
		if !isTokenRequest && user.Username != usernameFromReq {
			orgOwner, err := a.userStore.GetUserByID(ctx.Request().Context(), permissions.OrganizationID)
			if err != nil {
				return err
			}
			ctx.Set(string(types.UserContextKey), orgOwner)
		}

		// when error is nil, we should call handler func (the next middleware func)
		return nil
	}

	return fmt.Errorf("authentication details are missing")
}
