package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) getImageNamespace(ctx echo.Context) (string, error) {
	if ctx.Request().URL.Path == "/token" {
		scope, err := a.getScopeFromQueryParams(ctx.QueryParam("scope"))
		if err != nil {
			a.logger.Debug().Str("method", "getImageNamespace").Str("url", ctx.Request().URL.String()).Err(err).Send()
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
	l := len(authSchemeBasic)

	if len(auth) > l+1 && strings.EqualFold(auth[:l], authSchemeBasic) {
		b, err := base64.StdEncoding.DecodeString(auth[l+1:])
		if err != nil {
			a.logger.Debug().Err(err).Send()
			return err
		}
		cred := string(b)
		for i := 0; i < len(cred); i++ {
			if cred[i] == ':' {
				username, password := cred[:i], cred[i+1:]
				// Verify credentials
				if username == "" {
					errMsg := fmt.Errorf("username cannot be empty")
					a.logger.Debug().Err(errMsg).Send()
					return errMsg
				}

				if password == "" {
					errMsg := fmt.Errorf("password cannot be empty")
					a.logger.Debug().Err(errMsg).Send()
					return errMsg
				}

				userFromDb, err := a.pgStore.GetUserByUsername(context.Background(), username)
				if err != nil {
					a.logger.Debug().Err(err).Send()
					return err
				}
				if !a.verifyPassword(userFromDb.Password, password) {
					errMsg := fmt.Errorf("password is incorrect")
					a.logger.Debug().Err(errMsg).Send()
					return errMsg
				}
				ctx.Set(string(types.UserContextKey), userFromDb)
				break
			}
		}
	}

	return nil
}

func (a *auth) RepositoryPermissionsMiddleware() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if err := a.populateUserFromPermissionsCheck(ctx); err != nil {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeDenied,
					"invalid user credentials",
					echo.Map{
						"error": err.Error(),
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				a.logger.Log(ctx, err).Send()
				return echoErr
			}
			// handle skipping scenarios
			if ctx.QueryParam("offline_token") == "true" {
				a.logger.Log(ctx, nil).Bool("skipping_middleware", true).Str("request_type", "offline_token").Send()
				return handler(ctx)
			}

			namespace, err := a.getImageNamespace(ctx)
			if err != nil {
				errMsg := common.RegistryErrorResponse("UNKNOWN", "invalid image namespace", echo.Map{
					"error": err.Error(),
				})
				echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			usernameFromReq := strings.Split(namespace, "/")[0]
			repository, err := a.registryStore.GetRepositoryByNamespace(ctx.Request().Context(), namespace)
			if err == nil {
				if repository.Visibility == types.RepositoryVisibilityPublic {
					a.logger.Log(ctx, nil).Bool("skipping_middleware", true).Str("request_type", "public_pull").Send()
					return handler(ctx)
				}
			}

			user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
			if (!ok || user.Username != usernameFromReq) && usernameFromReq != types.RepositoryNameIPFS {
				errMsg := common.RegistryErrorResponse(
					registry.RegistryErrorCodeUnauthorized,
					"access to this resource is restricted, please login or check with the repository owner",
					echo.Map{
						"error": "authentication details are missing",
					},
				)
				echoErr := ctx.JSONBlob(http.StatusForbidden, errMsg.Bytes())
				a.logger.Log(ctx, errMsg).Send()
				return echoErr
			}

			a.logger.Log(ctx, nil).Send()
			return handler(ctx)
		}
	}
}
