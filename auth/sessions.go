package auth

import (
	"fmt"
	"net/http"
	"net/url"
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

	sessionCookie, err := ctx.Cookie("session_id")
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error while getting cookie",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error while parsing cookie",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("ERR_INVALID_COOKIE")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid cookie",
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

		if deleteAllSessions {
			err = a.sessionStore.DeleteAllSessions(ctx.Request().Context(), userId)
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

	if sessionUUID.String() != "" {
		err = a.sessionStore.DeleteSession(ctx.Request().Context(), sessionUUID, userId)
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
