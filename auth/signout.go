package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
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

	sessionUUID, err := uuid.Parse(parts[0])
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid session id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	userId, err := uuid.Parse(parts[1])
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid user id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = a.sessionStore.DeleteSession(ctx.Request().Context(), sessionUUID, userId); err != nil {
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
