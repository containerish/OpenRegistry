package auth

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

func (a *auth) ExpireSessions(ctx echo.Context) error {
	sessionId := ctx.QueryParam("session_id")
	deleteAll, err := strconv.ParseBool(ctx.QueryParam("delete_all"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "delete_all must be a boolean",
		})
	}
	if sessionId != "" {

	}
	return nil
}
