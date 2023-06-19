package auth

import (
	"fmt"
	"net/http"
	"net/url"
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
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "session cookie format is invalid",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid session id")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   "INVALID_SESSION_ID",
			"message": err,
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionUUID := parts[0]
	userId := parts[1]

	if err = a.pgStore.DeleteSession(ctx.Request().Context(), sessionUUID, userId); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not delete sessions",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	ctx.SetCookie(a.createCookie(ctx, "access_token", "", true, time.Now().Add(-time.Hour*750)))
	ctx.SetCookie(a.createCookie(ctx, "refresh_token", "", true, time.Now().Add(-time.Hour*750)))
	ctx.SetCookie(a.createCookie(ctx, "session_id", "", true, time.Now().Add(-time.Hour*750)))
	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "session deleted successfully",
	})
	a.logger.Log(ctx, err).Send()
	return err
}
