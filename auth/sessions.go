package auth

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) ExpireSessions(ctx echo.Context) error {
	//queryParamSessionId := ctx.QueryParam("session_id")
	cookie, err := ctx.Cookie("session_id")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "error while getting cookie",
		})
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid cookie",
		})
	}

	sessionID := parts[0]
	userId := parts[1]

	var deleteAllSessions bool
	queryParamDeleteAll := ctx.QueryParam("delete_all")
	if queryParamDeleteAll != "" {
		deleteAllSessions, err = strconv.ParseBool(queryParamDeleteAll)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "delete_all must be a boolean",
			})
		}
		_, err := uuid.Parse(userId)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "invalid user id",
			})
		}

		if deleteAllSessions {
			err := a.pgStore.DeleteAllSessions(ctx.Request().Context(), userId)
			if err != nil {
				return ctx.JSON(http.StatusInternalServerError, echo.Map{
					"error":   err.Error(),
					"message": "could not delete all sessions",
				})
			}
		}

	}

	if sessionID != "" {
		_, err := uuid.Parse(sessionID)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "invalid session id",
			})
		}
		err = a.pgStore.DeleteSession(ctx.Request().Context(), sessionID, userId)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "could not delete session",
			})
		}
	}

	return nil
}
