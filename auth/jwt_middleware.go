package auth

import (
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
		BeforeFunc:              middleware.DefaultJWTConfig.BeforeFunc,
		SuccessHandler:          middleware.DefaultJWTConfig.SuccessHandler,
		ErrorHandler:            middleware.DefaultJWTConfig.ErrorHandler,
		ErrorHandlerWithContext: middleware.DefaultJWTConfig.ErrorHandlerWithContext,
		KeyFunc:                 middleware.DefaultJWTConfig.KeyFunc,
		ParseTokenFunc:          middleware.DefaultJWTConfig.ParseTokenFunc,
		SigningKey:              []byte(a.c.SigningSecret),
		SigningKeys:             map[string]interface{}{},
		SigningMethod:           jwt.SigningMethodHS256.Name,
		Claims:                  &Claims{},
	})
}
