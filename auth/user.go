package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
)

func (a *auth) ReadUserWithSession(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		a.logger.Log(ctx).Send()
	}()

	cookie := ctx.QueryParam("session_id")
	if cookie == "" {
		ctx.Set(types.HttpEndpointErrorKey, "error in getting cookies")
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"msg": "error is cookie",
		})
	}
	parts := strings.Split(cookie, ":")
	if len(parts) != 2 {
		ctx.Set(types.HttpEndpointErrorKey, "invalid session id")
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid session id",
		})
	}
	sessionId := parts[0]
	color.Yellow("sessin in getSesion: %s", sessionId)

	user, err := a.pgStore.GetUserWithSession(ctx.Request().Context(), sessionId)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERROR_FETCHING_USER_WITH_SESSION",
		})
	}
	return ctx.JSON(http.StatusOK, user)
}
