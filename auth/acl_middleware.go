package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ACL implies a basic Access Control List on protected resources
func (a *auth) ACL() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Set(types.HandlerStartTime, time.Now())

			token, ok := ctx.Get("user").(*jwt.Token)
			if !ok {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, fmt.Errorf("ACL: unauthorized")).Send()
				return echoErr
			}

			claims, ok := token.Claims.(*OCIClaims)
			if !ok {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, fmt.Errorf("ACL: invalid claims")).Send()
				return echoErr
			}

			usernameFromReq := ctx.Param("username")
			userId, err := uuid.Parse(claims.ID)
			if err != nil {
				echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
					"error":   err.Error(),
					"message": "invalid user id format",
				})
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			user, err := a.userStore.GetUserByID(ctx.Request().Context(), userId)
			if err != nil {
				echoErr := ctx.NoContent(http.StatusUnauthorized)
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			ns := ctx.Param("username") + "/" + ctx.Param("imagename")
			permissions := a.permissionsStore.GetUserPermissionsForNamespace(ctx.Request().Context(), ns, user.ID)

			m := ctx.Request().Method
			readOp := m == http.MethodGet || m == http.MethodHead
			userIsOwner := usernameFromReq == user.Username || usernameFromReq == types.SystemUsernameIPFS
			permsAllowed := permissions.IsAdmin || (readOp && permissions.Pull) || (!readOp && permissions.Push)

			if permsAllowed || userIsOwner {
				a.logger.DebugWithContext(ctx).Str("middleware", "ACL").Bool("skip", true).Send()
				return handler(ctx)
			}
			//
			// if usernameFromReq == user.Username || usernameFromReq == types.RepositoryNameIPFS {
			// 	return handler(ctx)
			// }

			echoErr := ctx.NoContent(http.StatusUnauthorized)
			a.logger.DebugWithContext(ctx).Send()
			return echoErr
		}
	}
}
