package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

const (
	JWT_AUTH_KEY           = "JWT_AUTH"
	AuthorizationHeaderKey = "Authorization"
)

// BasicAuth returns a middleware which in turn can be used to perform http basic auth
func (a *auth) BasicAuth() echo.MiddlewareFunc {
	return a.BasicAuthWithConfig()
}

func (a *auth) buildBasicAuthenticationHeader(repoNamespace string) string {
	return fmt.Sprintf(
		"Bearer realm=%s,service=%s,scope=%s",
		strconv.Quote(fmt.Sprintf("%s/token", a.c.Endpoint())),
		strconv.Quote(a.c.Endpoint()),
		strconv.Quote(fmt.Sprintf("repository:%s:pull,push", repoNamespace)),
	)
}

func (a *auth) checkJWT(authHeader string, cookies []*http.Cookie) bool {
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 {
		return strings.EqualFold(parts[0], "Bearer")
	}

	// fallback to check for auth header in cookies
	for _, cookie := range cookies {
		if cookie.Name == AccessCookieKey {
			// early return if access_token is found in cookies, this will be checked by the JWT middlware and not the
			// basic auth middleware
			return true
		}
	}

	return false
}

const (
	defaultRealm    = "Restricted"
	authScheme      = "Bearer"
	authSchemeBasic = "Basic"
)

// BasicAuthConfig is a local copy of echo's middleware.BasicAuthWithConfig
func (a *auth) BasicAuthWithConfig() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if a.SkipBasicAuth(ctx) {
				a.logger.Debug().Bool("skip_basic_auth", true).Send()
				// Note: there might be other middlewares attached to this handler
				return handler(ctx)
			}

			auth := ctx.Request().Header.Get(echo.HeaderAuthorization)
			l := len(authScheme)

			if len(auth) > l+1 && strings.EqualFold(auth[:l], authScheme) {
				b, err := base64.StdEncoding.DecodeString(auth[l+1:])
				if err != nil {
					a.logger.Debug().Err(err).Send()
					return err
				}
				cred := string(b)
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Verify credentials
						valid, err := a.BasicAuthValidator(cred[:i], cred[i+1:], ctx)
						if err != nil {
							a.logger.Debug().Err(err).Send()
							return err
						} else if valid {
							return handler(ctx)
						}
						break
					}
				}
			}

			headerValue := fmt.Sprintf("Bearer realm=%s", strconv.Quote(fmt.Sprintf("%s/token", a.c.Endpoint())))
			username := ctx.Param("username")
			imageName := ctx.Param("imagename")
			if username != "" && imageName != "" {
				headerValue = a.buildBasicAuthenticationHeader(username + "/" + imageName)
			}

			// Need to return `401` for browsers to pop-up login box.
			ctx.Response().Header().Set(echo.HeaderWWWAuthenticate, headerValue)
			echoErr := ctx.NoContent(http.StatusUnauthorized)
			a.logger.Log(ctx, nil).Send()
			return echoErr
		}
	}
}

func (a *auth) BasicAuthValidator(username string, password string, ctx echo.Context) (bool, error) {
	ctx.Set(types.HandlerStartTime, time.Now())

	_, err := a.validateUser(username, password)
	usernameFromReq := ctx.Param("username")
	if err != nil || usernameFromReq != username {
		var errMsg registry.RegistryErrors
		errMsg.Errors = append(errMsg.Errors, registry.RegistryError{
			Code:    registry.RegistryErrorCodeUnauthorized,
			Message: "user is not authorized to perform this action",
			Detail: echo.Map{
				"reason": "you are not allowed to push to this account, please check if you are logged in with the right user.",
				"error":  err.Error(),
			},
		})
		echoErr := ctx.JSON(http.StatusUnauthorized, errMsg)
		a.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return false, echoErr
	}

	return true, nil
}

func (a *auth) SkipBasicAuth(ctx echo.Context) bool {
	authHeader := ctx.Request().Header.Get(echo.HeaderAuthorization)

	// if found, populate requested repository in request context, so that any of the chained middlwares can
	// read the value from ctx instead of database
	repo := a.tryPopulateRepository(ctx)

	// if Authorization header contains JWT, we skip basic auth and perform a JWT validation
	if ok := a.checkJWT(authHeader, ctx.Request().Cookies()); ok {
		a.logger.Debug().Bool("skip_basic_auth", true).Str("method", ctx.Request().Method).Str("path", ctx.Request().URL.RequestURI()).Send()
		return true
	}

	readOp := ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet
	// if it's a read operation on a public repository, we skip auth requirement
	if readOp && repo != nil && repo.Visibility == types.RepositoryVisibilityPublic {
		a.logger.Debug().Bool("skip_basic_auth", true).Str("method", ctx.Request().Method).Str("path", ctx.Request().URL.RequestURI()).Send()
		return true
	}

	return false
}

func (a *auth) tryPopulateRepository(ctx echo.Context) *types.ContainerImageRepository {
	if strings.HasPrefix(ctx.Request().URL.Path, "/v2/") {
		username := ctx.Param("username")
		imageName := ctx.Param("imagename")
		if username != "" && imageName != "" {
			ns := username + "/" + imageName
			repo, err := a.registryStore.GetRepositoryByNamespace(ctx.Request().Context(), ns)
			if err == nil {
				ctx.Set(string(types.UserRepositoryContextKey), repo)
				return repo
			}
		}
	}
	return nil
}
