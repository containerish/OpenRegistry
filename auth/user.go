package auth

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
)

func (a *auth) ReadUserWithSession(ctx echo.Context) error {
	cookie, err := ctx.Cookie("session_id")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "error is cookie",
		})
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid session id",
		})
	}
	sessionId := parts[0]

	user, err := a.pgStore.GetUserWithSession(ctx.Request().Context(), sessionId)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERROR_FETCHING_USER_WITH_SESSION",
		})
	}
	fmt.Printf("session details: id: %s owner:%s", user.Id, user.Username)
	return ctx.JSON(http.StatusOK, user)
}
