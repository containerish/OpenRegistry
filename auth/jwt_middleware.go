package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/store/v2/types"
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
			if strings.HasPrefix(ctx.Request().RequestURI, "/auth") {
				return false
			}

			if strings.HasPrefix(ctx.Request().RequestURI, "/v2/ext") {
				return false
			}

			// if JWT_AUTH is not set, we don't need to perform JWT authentication
			jwtAuth, ok := ctx.Get(JWT_AUTH_KEY).(bool)
			if !ok {
				a.logger.Debug().Str("method", "JWT").Str("type", "middleware").Bool("skip", true).Send()
				return true
			}

			if jwtAuth {
				return false
			}

			a.logger.Debug().Str("method", "JWT").Str("type", "middleware").Bool("skip", true).Send()
			return true
		},
		ErrorHandler: func(ctx echo.Context, err error) error {
			ctx.Set(types.HandlerStartTime, time.Now())
			echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "missing authentication information",
			})
			a.logger.Debug().Err(err).Send()
			a.logger.Log(ctx, err).Send()
			return echoErr
		},
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			return a.c.Registry.Auth.JWTSigningPubKey, nil
		},
		SuccessHandler: func(ctx echo.Context) {
			if token, tokenOk := ctx.Get("user").(*jwt.Token); tokenOk {
				claims, claimsOk := token.Claims.(*Claims)
				if claimsOk {
					user, _ := a.pgStore.GetUserByID(ctx.Request().Context(), uuid.MustParse(claims.ID))
					ctx.Set(string(types.UserClaimsContextKey), claims)
					ctx.Set(string(types.UserContextKey), user)
					a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", true).Send()
					return
				}
			}

			a.logger.Debug().Str("method", "jwt_success_handler").Bool("success", false).Send()
		},
		SigningKey:    a.c.Registry.Auth.JWTSigningPrivateKey,
		SigningKeys:   map[string]interface{}{},
		SigningMethod: jwt.SigningMethodRS256.Name,
		// Claims:         &Claims{},
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

			user, err := a.pgStore.GetUserByID(ctx.Request().Context(), uuid.MustParse(claims.ID))
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
		// Claims:         &Claims{},
		TokenLookup: fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
	})
}
