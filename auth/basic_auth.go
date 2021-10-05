package auth

import (
	"net/http"

	"github.com/containerish/OpenRegistry/registry/v2"
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
	return middleware.BasicAuth(func(username string, password string, ctx echo.Context) (bool, error) {

		if ctx.Request().RequestURI != "/v2/" {
			if ctx.Request().Method == http.MethodHead || ctx.Request().Method == http.MethodGet {
				return true, nil
			}
		}

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
	})
}
