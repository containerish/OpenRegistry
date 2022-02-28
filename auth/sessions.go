package auth

import (
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

func (a *auth) ExpireSessions(ctx echo.Context) error {
	queryParamSessionId := ctx.QueryParam("session_id")
	var sessionID uuid.UUID
	var err error
	if queryParamSessionId != "" {
		sessionID, err = uuid.Parse(queryParamSessionId)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "invalid session id",
			})
		}
	}
	var deleteAllSessions bool
	queryParamDeleteAll := ctx.QueryParam("delete_all")
	if queryParamDeleteAll != "" {
		deleteAllSessions, err = strconv.ParseBool(queryParamDeleteAll)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "delete_all must be a boolean",
			})
		}
	}

	if deleteAllSessions {
		user := ctx.Get("user").(*jwt.Token)
		claims := user.Claims.(*Claims)
		userId := claims.Id
		color.Red("came here, userId:%s:%s", sessionID, userId)

	}

	return nil
}
