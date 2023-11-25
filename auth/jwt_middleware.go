package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	echo_jwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

const (
	AccessCookieKey = "access_token"
	RefreshCookKey  = "refresh_token"
	QueryToken      = "token"
)

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWT() echo.MiddlewareFunc {
	return echo_jwt.WithConfig(echo_jwt.Config{
		Skipper: func(ctx echo.Context) bool {
			if _, ok := ctx.Get(types.HandlerStartTime).(*time.Time); !ok {
				// if handler timer isn't set, set one now
				ctx.Set(types.HandlerStartTime, time.Now())
			}

			// public read is allowed
			readOp := ctx.Request().Method == http.MethodGet || ctx.Request().Method == http.MethodHead
			// repository should be present at this step since the BasicAuth Middleware sets it
			repo, ok := ctx.Get(string(types.UserRepositoryContextKey)).(*types.ContainerImageRepository)
			if !ok {
				a.logger.Debug().Bool("repository_found_in_ctx", ok).Send()
				return false
			}

			skip := readOp && repo.Visibility == types.RepositoryVisibilityPublic
			if skip {
				a.logger.Debug().
					Bool("skip_jwt_middleware", true).
					Str("method", ctx.Request().Method).
					Str("path", ctx.Request().URL.RequestURI()).
					Send()
			}

			return skip
		},
		ErrorHandler: func(ctx echo.Context, err error) error {
			registryErr := registry.RegistryErrors{
				Errors: []registry.RegistryError{
					{
						Detail: echo.Map{
							"error": err.Error(),
						},
						Code:    registry.RegistryErrorCodeDenied,
						Message: "Missing authentication token",
					},
				}}

			echoErr := ctx.JSON(http.StatusUnauthorized, registryErr)
			a.logger.Log(ctx, err).Send()
			return echoErr
		},
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			return a.c.Registry.Auth.JWTSigningPubKey, nil
		},
		ContinueOnIgnoredError: false,
		SuccessHandler: func(ctx echo.Context) {

			if token, tokenOk := ctx.Get("user").(*jwt.Token); tokenOk {
				claims, claimsOk := token.Claims.(*Claims)
				if claimsOk {
					userId := uuid.MustParse(claims.ID)
					user, err := a.pgStore.GetUserByID(ctx.Request().Context(), userId)
					if err == nil {
						ctx.Set(string(types.UserContextKey), user)
					}
					ctx.Set(string(types.UserClaimsContextKey), claims)
					a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", true).Send()
					return
				}
			}

			a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", false).Send()
		},
		SigningKey:    a.c.Registry.Auth.JWTSigningPrivateKey,
		SigningMethod: jwt.SigningMethodRS256.Name,
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return &Claims{}
		},
		TokenLookup: fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
	})
}

// ACL implies a basic Access Control List on protected resources
func (a *auth) ACL() echo.MiddlewareFunc {
	return func(hf echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Set(types.HandlerStartTime, time.Now())

			m := ctx.Request().Method
			if m == http.MethodGet || m == http.MethodHead {
				return hf(ctx)
			}

			token, ok := ctx.Get("user").(*jwt.Token)
			if !ok {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, fmt.Errorf("ACL: unauthorized")).Send()
				return echoErr
			}

			claims, ok := token.Claims.(*Claims)
			if !ok {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, fmt.Errorf("ACL: invalid claims")).Send()
				return echoErr
			}

			username := ctx.Param("username")

			userId, err := uuid.Parse(claims.ID)
			if err != nil {
				echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
					"error":   err.Error(),
					"message": "invalid user id format",
				})
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			user, err := a.pgStore.GetUserByID(ctx.Request().Context(), userId)
			if err != nil {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, err).Send()
				return echoErr
			}
			if user.Username == username {
				return hf(ctx)
			}

			return ctx.NoContent(http.StatusUnauthorized)

		}
	}
}

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWTRest() echo.MiddlewareFunc {
	return echo_jwt.WithConfig(echo_jwt.Config{
		Skipper: func(ctx echo.Context) bool {
			// this is a signin request from the client
			if ctx.Request().URL.Path == "/token" && ctx.QueryParam("offline_token") == "true" {
				return true
			}

			return false
		},
		ErrorHandler: func(ctx echo.Context, err error) error {
			ctx.Set(types.HandlerStartTime, time.Now())

			echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "missing authentication information",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		},
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			return a.c.Registry.Auth.JWTSigningPubKey, nil
		},

		SigningKey:    a.c.Registry.Auth.JWTSigningPrivateKey,
		SigningMethod: jwt.SigningMethodRS256.Name,
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return &Claims{}
		},
		SuccessHandler: func(ctx echo.Context) {
			if token, tokenOk := ctx.Get("user").(*jwt.Token); tokenOk {
				claims, claimsOk := token.Claims.(*Claims)
				if claimsOk {
					userId := uuid.MustParse(claims.ID)
					user, err := a.pgStore.GetUserByID(ctx.Request().Context(), userId)
					if err == nil {
						ctx.Set(string(types.UserContextKey), user)
					}
					ctx.Set(string(types.UserClaimsContextKey), claims)
					a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", true).Send()
					return
				}
			}

			a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", false).Send()
		},
		TokenLookup: fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
	})
}
