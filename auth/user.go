package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) ReadUserWithSession(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	session, err := ctx.Cookie("session_id")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERROR_GETTING_SESSION_ID",
		})
	}
	if session.Value == "" {
		a.logger.Log(ctx, fmt.Errorf("error getting cookies"))
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"msg": "error is cookie",
		})
	}

	parts := strings.Split(session.Value, ":")
	if len(parts) != 2 {
		a.logger.Log(ctx, fmt.Errorf("invalid session id"))
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid session id",
		})
	}

	sessionId := parts[0]
	user, err := a.pgStore.GetUserWithSession(ctx.Request().Context(), sessionId)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERROR_FETCHING_USER_WITH_SESSION",
		})
	}
	return ctx.JSON(http.StatusOK, user)
}
