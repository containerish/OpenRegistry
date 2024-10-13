package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	echo_jwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWT() echo.MiddlewareFunc {
	return echo_jwt.WithConfig(echo_jwt.Config{
		Skipper: func(ctx echo.Context) bool {
			if _, ok := ctx.Get(types.HandlerStartTime).(*time.Time); !ok {
				// if handler timer isn't set, set one now
				ctx.Set(types.HandlerStartTime, time.Now())
			}

			if strings.HasPrefix(ctx.Request().URL.Path, "/v2/ext/") {
				return true
			}

			// public read is allowed
			readOp := ctx.Request().Method == http.MethodGet || ctx.Request().Method == http.MethodHead
			// repository should be present at this step since the BasicAuth Middleware sets it
			repo, ok := ctx.Get(string(types.UserRepositoryContextKey)).(*types.ContainerImageRepository)
			if !ok {
				return false
			}

			repoName := ctx.Param("imagename")

			skip := (readOp && repo.Visibility == types.RepositoryVisibilityPublic) ||
				repoName == types.SystemUsernameIPFS
			if skip {
				a.logger.DebugWithContext(ctx).Bool("skip_jwt_middleware", true).Send()
			}

			return skip
		},
		ErrorHandler: func(ctx echo.Context, err error) error {
			registryErr := common.RegistryErrorResponse(
				registry.RegistryErrorCodeDenied,
				"Missing authentication token",
				echo.Map{
					"error": err.Error(),
				},
			)

			echoErr := ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
			a.logger.DebugWithContext(ctx).Err(err).Send()
			return echoErr
		},
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			return a.c.Registry.Auth.JWTSigningPubKey, nil
		},
		ContinueOnIgnoredError: false,
		SuccessHandler: func(ctx echo.Context) {
			if token, tokenOk := ctx.Get("user").(*jwt.Token); tokenOk {
				if claims, claimsOk := token.Claims.(*OCIClaims); claimsOk {
					var (
						user *types.User
						err  error
					)
					userId := uuid.MustParse(claims.ID)
					usernameFromReq := ctx.Param("username")
					if usernameFromReq == types.SystemUsernameIPFS {
						user, err = a.userStore.GetIPFSUser(ctx.Request().Context())
					} else {
						user, err = a.userStore.GetUserByID(ctx.Request().Context(), userId)
					}

					if err == nil {
						ctx.Set(string(types.UserContextKey), user)
					}
					ctx.Set(string(types.UserClaimsContextKey), claims)
					a.logger.DebugWithContext(ctx).Bool("success", true).Send()
					return
				}
			}

			a.logger.DebugWithContext(ctx).Bool("success", false).Send()
		},
		SigningKey:    a.c.Registry.Auth.JWTSigningPrivateKey,
		SigningMethod: jwt.SigningMethodRS256.Name,
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return &OCIClaims{}
		},
		TokenLookup: fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
	})
}
