package router

import (
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/registry/v2"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

func registryNamespaceValidator(logger telemetry.Logger) echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			// we skip the /v2/ path since it isn't a namespaced path
			if ctx.Request().URL.Path == "/v2/" {
				return handler(ctx)
			}

			namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
			if namespace != "/" && !config.RegistryNSRegex.MatchString(namespace) {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeNameInvalid,
					"invalid user namespace",
					echo.Map{
						"error": "the required format for namespace is <username>/<imagename>",
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				logger.DebugWithContext(ctx).Err(registryErr).Send()
				return echoErr
			}

			ctx.Set(string(registry.RegistryNamespace), namespace)
			return handler(ctx)
		}
	}
}

func registryReferenceOrTagValidator(logger telemetry.Logger) echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ref := ctx.Param("reference")
			if ref == "" || !config.RegistryManifestRefRegex.MatchString(ref) {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeTagInvalid,
					"reference/tag does not match the required format",
					echo.Map{
						"error": fmt.Sprintf(
							"reference/tag must match the following regex: %s",
							config.RegistryManifestRefRegex.String(),
						),
					},
				)

				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				logger.DebugWithContext(ctx).Err(registryErr).Send()
				return echoErr
			}

			return handler(ctx)
		}
	}
}

func propagateRepository(store registry_store.RegistryStore, logger telemetry.Logger) echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			imageName := ctx.Param("imagename")
			user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
			if !ok {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeUnauthorized,
					"Unauthorized",
					echo.Map{
						"error": "User is not found in request context",
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				logger.DebugWithContext(ctx).Err(registryErr).Send()
				return echoErr
			}

			repository, err := store.GetRepositoryByName(ctx.Request().Context(), user.ID, imageName)
			if err == nil {
				ctx.Set(string(types.UserRepositoryContextKey), repository)
			}

			return handler(ctx)
		}
	}
}
