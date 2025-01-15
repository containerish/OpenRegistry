package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) ReadUserWithSession(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	sessionCookie, err := ctx.Cookie("session_id")
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting session id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing session id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("INVALID_SESSION_ID")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid session id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionUUID := parts[0]
	user, err := a.userStore.GetUserWithSession(ctx.Request().Context(), sessionUUID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting user with session",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.JSON(http.StatusOK, user)
	a.logger.Log(ctx, err).Send()
	return err
}
