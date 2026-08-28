package auth

import (
	"context"
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
				// Note: there might be other middlewares attached to this handler
				return handler(ctx)
			}

			auth := ctx.Request().Header.Get(echo.HeaderAuthorization)
			// auth should have something like "Basic username:password"
			if len(auth) > len(authSchemeBasic)+1 && strings.EqualFold(auth[:len(authSchemeBasic)], authSchemeBasic) {
				user, err := a.validateBasicAuthCredentials(auth)
				if err != nil {
					registryErr := common.RegistryErrorResponse(
						registry.RegistryErrorCodeDenied,
						"invalid credentials",
						echo.Map{
							"error": err.Error(),
						},
					)

					echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
					a.logger.DebugWithContext(ctx).Err(registryErr).Send()
					return echoErr
				}
				ctx.Set(string(types.UserContextKey), user)
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

	if strings.HasPrefix(ctx.Request().URL.Path, "/v2/ext/") {
		return true
	}

	hasJWT := a.checkJWT(authHeader, ctx.Request().Cookies())
	if hasJWT {
		return true
	}
	// if found, populate requested repository in request context, so that any of the chained middlwares can
	// read the value from ctx instead of database
	repo := a.tryPopulateRepository(ctx)
	if !hasJWT || repo == nil {
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
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	isOCIRequest := strings.HasPrefix(ctx.Request().URL.Path, "/v2/") && namespace != "/"
	if isOCIRequest {
		repo, err := a.registryStore.GetRepositoryByNamespace(ctx.Request().Context(), namespace)
		if err == nil {
			ctx.Set(string(types.UserRepositoryContextKey), repo)
			return repo
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

	user, err := a.validateUser(username, password)
	if err != nil {
		return err
	}

	ctx.Set(string(types.UserContextKey), user)

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

func (a *auth) validateBasicAuthCredentials(auth string) (*types.User, error) {
	l := len(authSchemeBasic)

	if len(auth) > l+1 && strings.EqualFold(auth[:l], authSchemeBasic) {
		decodedCredentials, err := base64.StdEncoding.DecodeString(auth[l+1:])
		if err != nil {
			return nil, err
		}

		basicAuthCredentials := strings.Split(string(decodedCredentials), ":")
		username, password := basicAuthCredentials[0], basicAuthCredentials[1]

		// try login with GitHub PAT
		// 1. "github_pat_" prefix is for the new fine-grained, repo scoped tokens
		// 2. "ghp_" prefix is for the old (classic) github tokens
		if strings.HasPrefix(password, "github_pat_") || strings.HasPrefix(password, "ghp_") {
			user, ghErr := a.getUserWithGithubOauthToken(context.Background(), password)
			if ghErr != nil {
				return nil, fmt.Errorf("ERR_READ_USER_WITH_GITHUB_TOKEN: %w", ghErr)
			}

			return user, nil
		}

		if strings.HasPrefix(password, types.OpenRegistryAuthTokenPrefix) {
			user, err := a.validateUserWithPAT(context.Background(), username, password)
			if err != nil {
				return nil, err
			}

			return user, nil
		}

		user, err := a.validateUser(username, password)
		if err != nil {
			return nil, err
		}

		return user, nil
	}

	return nil, fmt.Errorf("missing basic authentication credentials")

}
