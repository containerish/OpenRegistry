package auth

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	return BasicAuthWithConfig(middleware.BasicAuthConfig{
		Skipper: func(ctx echo.Context) bool {
			if ctx.Request().RequestURI != "/v2/" {
				if ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet {
					return true
				}
			}

			if ctx.Request().RequestURI == "/v2/" {
				return false
			}

			return false
		},
		Validator: func(username string, password string, ctx echo.Context) (bool, error) {
			color.Red("sljkdflksdjklsjf")
			if ctx.Request().RequestURI == "/v2/" {
				_, err := a.validateUser(username, password)
				if err != nil {
					return false, ctx.NoContent(http.StatusUnauthorized)
				}

				return true, nil
			}

			usernameFromNameSpace := ctx.Param("username")

			if usernameFromNameSpace != username {
				var errMsg registry.RegistryErrors
				errMsg.Errors = append(errMsg.Errors, registry.RegistryError{
					Code:    registry.RegistryErrorCodeDenied,
					Message: "not authorised",
					Detail:  nil,
				})
				return false, ctx.JSON(http.StatusForbidden, errMsg)
			}
			resp, err := a.validateUser(username, password)
			if err != nil {
				return false, err
			}

			ctx.Set("basic_auth", resp)
			return true, nil
		},
		Realm: "http://100.126.24.85:5000/token",
	})
}

const (
	defaultRealm = "Restricted"
	basic        = "Bearer"
)

func BasicAuthWithConfig(config middleware.BasicAuthConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Validator == nil {
		panic("echo: basic-auth middleware requires a validator function")
	}
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultBasicAuthConfig.Skipper
	}
	if config.Realm == "" {
		config.Realm = defaultRealm
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			auth := c.Request().Header.Get(echo.HeaderAuthorization)
			l := len(basic)

			if len(auth) > l+1 && strings.EqualFold(auth[:l], basic) {
				b, err := base64.StdEncoding.DecodeString(auth[l+1:])
				if err != nil {
					return err
				}
				cred := string(b)
				for i := 0; i < len(cred); i++ {
					if cred[i] == ':' {
						// Verify credentials
						valid, err := config.Validator(cred[:i], cred[i+1:], c)
						if err != nil {
							return err
						} else if valid {
							return next(c)
						}
						break
					}
				}
			}

			realm := defaultRealm
			if config.Realm != defaultRealm {
				realm = strconv.Quote(config.Realm)
			}

			// Need to return `401` for browsers to pop-up login box.
			c.Response().Header().Set(echo.HeaderWWWAuthenticate, basic+" realm="+realm)
			return echo.ErrUnauthorized
		}
	}
}
