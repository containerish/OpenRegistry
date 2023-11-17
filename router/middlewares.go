package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/containerish/OpenRegistry/registry/v2"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
	dist_spec "github.com/opencontainers/distribution-spec/specs-go/v1"
)

func registryNamespaceValidator() echo.MiddlewareFunc {
	// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	nsRegex := regexp.MustCompile(`[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*`)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			username := ctx.Param("username")
			imageName := ctx.Param("imagename")

			namespace := username + "/" + imageName
			if username == "" || imageName == "" || !nsRegex.MatchString(namespace) {
				registryErr := dist_spec.ErrorResponse{
					Errors: []dist_spec.ErrorInfo{
						{
							Code:    registry.RegistryErrorCodeNameInvalid,
							Message: "invalid user namespace",
							Detail:  "the required format for namespace is <username>/<imagename>",
						},
					},
				}
				errBz, _ := json.Marshal(registryErr)
				return ctx.JSONBlob(http.StatusBadRequest, errBz)
			}

			ctx.Set(string(registry.RegistryNamespace), namespace)
			return next(ctx)
		}
	}
}

func registryReferenceOrTagValidator() echo.MiddlewareFunc {
	// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	refRegex := regexp.MustCompile(`[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}`)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ref := ctx.Param("reference")

			if ref != "" && !refRegex.MatchString(ref) {
				err := dist_spec.ErrorResponse{
					Errors: []dist_spec.ErrorInfo{
						{
							Code:    registry.RegistryErrorCodeTagInvalid,
							Message: "reference/tag does not match the required format",
							Detail:  fmt.Sprintf("reference/tag must match the following regex: %s", refRegex.String()),
						},
					},
				}

				errBz, _ := json.Marshal(err)
				return ctx.JSONBlob(http.StatusBadRequest, errBz)
			}

			return next(ctx)
		}
	}
}

func progagatRepository(store registry_store.RegistryStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			imageName := ctx.Param("imagename")

			user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
			if !ok {
				registryErr := dist_spec.ErrorResponse{
					Errors: []dist_spec.ErrorInfo{
						{
							Code:    registry.RegistryErrorCodeUnauthorized,
							Message: "Unauthorized",
							Detail:  "User is not found in request context",
						},
					},
				}
				errBz, _ := json.Marshal(registryErr)
				return ctx.JSONBlob(http.StatusBadRequest, errBz)
			}

			repository, err := store.GetRepositoryByName(ctx.Request().Context(), user.ID, imageName)
			if err == nil {
				ctx.Set(string(types.UserRepositoryContextKey), repository)
			}

			return next(ctx)
		}
	}
}
