package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) SignOut(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	sessionCookie, err := ctx.Cookie("session_id")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting session ID fro sign-out user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	parts := strings.Split(sessionCookie.Value, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid session id")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   "INVALID_SESSION_ID",
			"message": err,
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionId := parts[0]
	userId := parts[1]

	if err = a.pgStore.DeleteSession(ctx.Request().Context(), sessionId, userId); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not delete sessions",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	ctx.SetCookie(a.createCookie("access", "", true, time.Now().Add(-time.Hour)))
	ctx.SetCookie(a.createCookie("refresh", "", true, time.Now().Add(-time.Hour)))
	ctx.SetCookie(a.createCookie("session_id", "", true, time.Now().Add(-time.Hour)))
	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "session deleted successfully",
	})
	a.logger.Log(ctx, err)
	return err
}
