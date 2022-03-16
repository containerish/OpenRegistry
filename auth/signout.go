package auth

import (
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
	"time"
)

func (a *auth) SignOut(ctx echo.Context) error {
	sessionCookie, err := ctx.Cookie("session_id")
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERROR_GETTING_SESSION_ID_FOR_SIGN_OUT",
		})
	}
	parts := strings.Split(sessionCookie.Value, ":")
	if len(parts) != 2 {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "INVALID_SESSION_ID",
		})
	}

	sessionId := parts[0]
	userId := parts[1]

	if err := a.pgStore.DeleteSession(ctx.Request().Context(), sessionId, userId); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not delete sessions",
		})
	}

	ctx.SetCookie(a.createCookie("access", "", true, time.Now().Add(-time.Hour)))
	ctx.SetCookie(a.createCookie("refresh", "", true, time.Now().Add(-time.Hour)))
	ctx.SetCookie(a.createCookie("session_id", "", true, time.Now().Add(-time.Hour)))
	return ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "session deleted successfully",
	})
}
