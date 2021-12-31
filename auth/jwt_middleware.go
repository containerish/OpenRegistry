package auth

import (
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWT() echo.MiddlewareFunc {
	return middleware.JWTWithConfig(middleware.JWTConfig{
		Skipper: func(ctx echo.Context) bool {
			// if JWT_AUTH is not set, we don't need to perform JWT authentication
			jwtAuth, ok := ctx.Get(JWT_AUTH_KEY).(bool)
			if !ok {
				return true
			}

			if jwtAuth {
				return false
			}

			return true
		},
		BeforeFunc:     middleware.DefaultJWTConfig.BeforeFunc,
		SuccessHandler: middleware.DefaultJWTConfig.SuccessHandler,
		ErrorHandler:   middleware.DefaultJWTConfig.ErrorHandler,
		ErrorHandlerWithContext: func(err error, ctx echo.Context) error {
			// ErrorHandlerWithContext only logs the failing requtest
			ctx.Set(types.HandlerStartTime, time.Now())
			ctx.Set(types.HttpEndpointErrorKey, err.Error())
			a.logger.Log(ctx)
			return ctx.NoContent(http.StatusUnauthorized)
		},
		KeyFunc:        middleware.DefaultJWTConfig.KeyFunc,
		ParseTokenFunc: middleware.DefaultJWTConfig.ParseTokenFunc,
		SigningKey:     []byte(a.c.Registry.SigningSecret),
		SigningKeys:    map[string]interface{}{},
		SigningMethod:  jwt.SigningMethodHS256.Name,
		Claims:         &Claims{},
	})
}

// ACL implies a basic Access Control List on protected resources
func (a *auth) ACL() echo.MiddlewareFunc {
	return func(hf echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Set(types.HandlerStartTime, time.Now())
			defer func() {
				a.logger.Log(ctx)
			}()

			m := ctx.Request().Method
			if m == http.MethodGet || m == http.MethodHead {
				return hf(ctx)
			}

			token, ok := ctx.Get("user").(*jwt.Token)
			if !ok {
				ctx.Set(types.HttpEndpointErrorKey, "ACL: unauthorized")
				return ctx.NoContent(http.StatusUnauthorized)
			}

			claims, ok := token.Claims.(*Claims)
			if !ok {
				ctx.Set(types.HttpEndpointErrorKey, "ACL: invalid claims")
				return ctx.NoContent(http.StatusUnauthorized)
			}

			username := ctx.Param("username")
			if claims.Subject == username {
				return hf(ctx)
			}

			ctx.Set(types.HttpEndpointErrorKey, "ACL: username didn't match from token")
			return ctx.NoContent(http.StatusUnauthorized)
		}
	}
}
