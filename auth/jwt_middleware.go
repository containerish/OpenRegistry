package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	echo_jwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

const (
	AccessCookieKey = "access_token"
	RefreshCookKey  = "refresh_token"
	Service         = "service"
	QueryToken      = "token"
)

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWTRest() echo.MiddlewareFunc {
	return echo_jwt.WithConfig(echo_jwt.Config{
		Skipper: func(ctx echo.Context) bool {
			// this is a signin request from the client
			if ctx.Request().URL.Path == "/token" && ctx.QueryParam("offline_token") == "true" {
				return true
			}

			return false
		},
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
		SuccessHandler: func(ctx echo.Context) {
			if token, tokenOk := ctx.Get("user").(*jwt.Token); tokenOk {
				claims, claimsOk := token.Claims.(*Claims)
				if claimsOk {
					userId := uuid.MustParse(claims.ID)
					user, err := a.userStore.GetUserByID(ctx.Request().Context(), userId)
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
		TokenLookup: fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
	})
}
