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
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting session id",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	if session.Value == "" {
		err = fmt.Errorf("ERR_GETTING_COOKIE")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting cookie",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	parts := strings.Split(session.Value, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("INVALID_SESSION_ID")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid session id",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionId := parts[0]
	user, err := a.pgStore.GetUserWithSession(ctx.Request().Context(), sessionId)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting user with session",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	err = ctx.JSON(http.StatusOK, user)
	a.logger.Log(ctx, err)
	return err
}
