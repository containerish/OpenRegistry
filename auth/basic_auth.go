package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

const (
	JWT_AUTH_KEY           = "JWT_AUTH"
	AuthorizationHeaderKey = "Authorization"
	defaultRealm           = "Restricted"
	authSchemeBearer       = "Bearer"
	authSchemeBasic        = "Basic"
)

// BasicAuth returns a middleware which in turn can be used to perform http basic auth
func (a *auth) BasicAuth() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {

			if a.SkipBasicAuth(ctx) {
				a.logger.DebugWithContext(ctx).Bool("skip_basic_auth", true).Send()
				// Note: there might be other middlewares attached to this handler
				return handler(ctx)
			}

			auth := ctx.Request().Header.Get(echo.HeaderAuthorization)
			l := len(authSchemeBasic)

			if len(auth) > l+1 && strings.EqualFold(auth[:l], authSchemeBasic) {
				b, err := base64.StdEncoding.DecodeString(auth[l+1:])
				if err != nil {
					registryErr := common.RegistryErrorResponse(
						registry.RegistryErrorCodeDenied,
						"invalid credentials",
						echo.Map{
							"error": err.Error(),
						},
					)

					echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
					a.logger.DebugWithContext(ctx).Err(err).Send()
					return echoErr
				}
				cred := string(b)
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Verify credentials
						if err = a.BasicAuthValidator(cred[:i], cred[i+1:], ctx); err != nil {
							registryErr := common.RegistryErrorResponse(
								registry.RegistryErrorCodeUnauthorized,
								"user is not authorized to perform this action",
								echo.Map{
									"reason": "Unauthorized. Please check if you are logged in with the right user.",
									"error":  err.Error(),
								},
							)
							echoErr := ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
							a.logger.DebugWithContext(ctx).Err(registryErr).Send()
							return echoErr
						}

						return handler(ctx)
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
			a.logger.Log(ctx, nil).Str(echo.HeaderWWWAuthenticate, headerValue).Send()
			return echoErr
		}
	}
}

func (a *auth) SkipBasicAuth(ctx echo.Context) bool {
	authHeader := ctx.Request().Header.Get(echo.HeaderAuthorization)

	hasJWT := a.checkJWT(authHeader, ctx.Request().Cookies())
	if hasJWT {
		return true
	}
	// if found, populate requested repository in request context, so that any of the chained middlwares can
	// read the value from ctx instead of database
	repo := a.tryPopulateRepository(ctx)
	if repo == nil {
		return false
	}

	// if Authorization header contains JWT, we skip basic auth and perform a JWT validation
	isIPFSRepo := ctx.Param("imagename") == types.SystemUsernameIPFS
	readOp := ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet
	// only skip now if one of the following cases match:
	// 1. It's a public pulls (IPFS pulls are always public)
	// 2. The request contains a JWT, in which case, we let the JWT middleware handle validation + scoping
	return readOp && (isIPFSRepo || repo.Visibility == types.RepositoryVisibilityPublic)
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

func (a *auth) checkJWT(authHeader string, cookies []*http.Cookie) bool {
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 {
		return strings.EqualFold(parts[0], authSchemeBearer)
	}

	// fallback to check for auth header in cookies
	for _, cookie := range cookies {
		if cookie.Name == AccessCookieKey || cookie.Name == QueryToken {
			// early return if access_token or token is found in cookies,
			// this will be checked by the JWT middlware and not the basic auth middleware
			return true
		}
	}

	return false
}

func (a *auth) BasicAuthValidator(username string, password string, ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	// readOp := ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet

	// usernameFromReq := ctx.Param("username")

	err := a.validateUser(username, password)
	if err != nil {
		// if err != nil || usernameFromReq != username || usernameFromReq == types.RepositoryNameIPFS {
		return err
	}

	return nil
}

func (a *auth) buildBasicAuthenticationHeader(repoNamespace string) string {
	return fmt.Sprintf(
		"Bearer realm=%s,service=%s,scope=%s",
		strconv.Quote(fmt.Sprintf("%s/token", a.c.Endpoint())),
		strconv.Quote(a.c.Endpoint()),
		strconv.Quote(fmt.Sprintf("repository:%s:pull,push", repoNamespace)),
	)
}
