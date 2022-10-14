package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) ExpireSessions(ctx echo.Context) error {
	//queryParamSessionId := ctx.QueryParam("session_id")
	ctx.Set(types.HandlerStartTime, time.Now())

	cookie, err := ctx.Cookie("session_id")
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error while getting cookie",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("ERR_INVALID_COOKIE")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid cookie",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionID := parts[0]
	userId := parts[1]

	var deleteAllSessions bool
	queryParamDeleteAll := ctx.QueryParam("delete_all")
	if queryParamDeleteAll != "" {
		deleteAllSessions, err = strconv.ParseBool(queryParamDeleteAll)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "delete_all must be a boolean",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}
		_, err = uuid.Parse(userId)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "invalid user id",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		if deleteAllSessions {
			err = a.pgStore.DeleteAllSessions(ctx.Request().Context(), userId)
			if err != nil {
				echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
					"error":   err.Error(),
					"message": "could not delete all sessions",
				})
				a.logger.Log(ctx, err).Send()
				return echoErr
			}
		}

	}

	if sessionID != "" {
		_, err = uuid.Parse(sessionID)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "invalid session id",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}
		err = a.pgStore.DeleteSession(ctx.Request().Context(), sessionID, userId)
		if err != nil {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "could not delete session",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}
	}

	err = ctx.NoContent(http.StatusAccepted)
	a.logger.Log(ctx, err).Send()
	return err
}
