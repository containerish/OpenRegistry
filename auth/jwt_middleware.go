package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	echo_jwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

const (
	AccessCookieKey  = "access_token"
	SessionCookieKey = "session_id"
	RefreshCookKey   = "refresh_token"
	Service          = "service"
	QueryToken       = "token"
)

// JWT basically uses the default JWT middleware by echo, but has a slightly different skipper func
func (a *auth) JWTRest() echo.MiddlewareFunc {
	return echo_jwt.WithConfig(echo_jwt.Config{
		Skipper: func(ctx echo.Context) bool {
			p := ctx.Request().URL.Path
			isPublicRepoDetailApi := p == "/v2/ext/catalog/repository" && ctx.QueryParam("public") == "true"
			return p == "/v2/ext/catalog/detail" || p == "/v2/_catalog/public" || isPublicRepoDetailApi
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
		KeyFunc: func(t *jwt.Token) (interface{}, error) {
			return a.c.Registry.Auth.JWTSigningPubKey, nil
		},
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return &Claims{}
		},
		TokenLookup:   fmt.Sprintf("cookie:%s,header:%s:Bearer ", AccessCookieKey, echo.HeaderAuthorization),
		SigningKey:    a.c.Registry.Auth.JWTSigningPrivateKey,
		SigningMethod: jwt.SigningMethodRS256.Name,
	})
}
