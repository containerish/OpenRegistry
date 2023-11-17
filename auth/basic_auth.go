package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

const (
	JWT_AUTH_KEY           = "JWT_AUTH"
	AuthorizationHeaderKey = "Authorization"
)

//when we use JWT
/*AuthMiddleware
HTTP/1.1 401 Unauthorized
Content-Type: application/json; charset=utf-8
Docker-Distribution-Api-Version: registry/2.0
Www-Authenticate: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",
scope="repository:samalba/my-app:pull,push"
Date: Thu, 10 Sep 2015 19:32:31 GMT
Content-Length: 235
Strict-Transport-Security: max-age=31536000

{"errors":[{"code":"UNAUTHORIZED","message":"","detail":}]}
*/
//var wwwAuthenticate = `Bearer realm="http://0.0.0.0:5000/auth/token",
//service="http://0.0.0.0:5000",scope="repository:%s`

// BasicAuth returns a middleware which in turn can be used to perform http basic auth
func (a *auth) BasicAuth() echo.MiddlewareFunc {
	return a.BasicAuthWithConfig()
}

func (a *auth) buildBasicAuthenticationHeader(repoNamespace string) string {
	return fmt.Sprintf(
		"Bearer realm=%s,service=%s,scope=repository:%s:pull,push",
		strconv.Quote(fmt.Sprintf("%s/token", a.c.Endpoint())),
		strconv.Quote(a.c.Endpoint()),
		strconv.Quote(fmt.Sprintf("repository:%s:pull,push", repoNamespace)),
	)
}

func (a *auth) checkJWT(authHeader string, cookies []*http.Cookie) bool {
	for _, cookie := range cookies {
		if cookie.Name == AccessCookieKey {
			// early return if access_token is found in cookies
			return true
		}
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		return false
	}

	return strings.EqualFold(parts[0], "Bearer")
}

const (
	defaultRealm = "Restricted"
	authScheme   = "Bearer"
)

// BasicAuthConfig is a local copy of echo's middleware.BasicAuthWithConfig
func (a *auth) BasicAuthWithConfig() echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {

			if a.SkipBasicAuth(ctx) {
				return next(ctx)
			}

			auth := ctx.Request().Header.Get(echo.HeaderAuthorization)
			l := len(authScheme)

			if len(auth) > l+1 && strings.EqualFold(auth[:l], authScheme) {
				b, err := base64.StdEncoding.DecodeString(auth[l+1:])
				if err != nil {
					return err
				}
				cred := string(b)
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Verify credentials
						valid, err := a.BasicAuthValidator(cred[:i], cred[i+1:], ctx)
						if err != nil {
							return err
						} else if valid {
							return next(ctx)
						}
						break
					}
				}
			}

			headerValue := fmt.Sprintf("Bearer realm=%s", strconv.Quote(fmt.Sprintf("%s/token", a.c.Endpoint())))
			namespace, ok := ctx.Get(string(registry.RegistryNamespace)).(string)
			if ok {
				headerValue = a.buildBasicAuthenticationHeader(namespace)
			}

			// Need to return `401` for browsers to pop-up login box.
			ctx.Response().Header().Set(echo.HeaderWWWAuthenticate, headerValue)
			return echo.ErrUnauthorized
		}
	}
}

func (a *auth) BasicAuthValidator(username string, password string, ctx echo.Context) (bool, error) {
	ctx.Set(types.HandlerStartTime, time.Now())

	if ctx.Request().URL.Path == "/v2/" {
		_, err := a.validateUser(username, password)
		if err != nil {
			echoErr := ctx.NoContent(http.StatusUnauthorized)
			a.logger.Log(ctx, err).Send()
			return false, echoErr
		}

		return true, nil
	}

	usernameFromNameSpace := ctx.Param("username")
	if usernameFromNameSpace != username {
		var errMsg registry.RegistryErrors
		errMsg.Errors = append(errMsg.Errors, registry.RegistryError{
			Code:    registry.RegistryErrorCodeDenied,
			Message: "user is not authorised to perform this action",
			Detail:  nil,
		})
		echoErr := ctx.JSON(http.StatusForbidden, errMsg)
		a.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return false, echoErr
	}
	_, err := a.validateUser(username, password)
	if err != nil {
		var errMsg registry.RegistryErrors
		errMsg.Errors = append(errMsg.Errors, registry.RegistryError{
			Code:    registry.RegistryErrorCodeDenied,
			Message: err.Error(),
			Detail:  nil,
		})
		echoErr := ctx.JSON(http.StatusUnauthorized, errMsg)
		a.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return false, echoErr
	}

	return true, nil
}

func (a *auth) SkipBasicAuth(ctx echo.Context) bool {
	authHeader := ctx.Request().Header.Get(AuthorizationHeaderKey)

	// if Authorization header contains JWT, we skip basic auth and perform a JWT validation
	if ok := a.checkJWT(authHeader, ctx.Request().Cookies()); ok {
		ctx.Set(JWT_AUTH_KEY, true)
		return true
	}

	if ctx.Request().URL.Path != "/v2/" {
		if ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet {
			return true
		}
	}

	if ctx.Request().URL.Path == "/v2/" {
		return false
	}

	return false
}
