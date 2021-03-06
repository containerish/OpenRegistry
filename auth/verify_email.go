package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) VerifyEmail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	token := ctx.QueryParam("token")
	if token == "" {
		err := fmt.Errorf("EMPTY_TOKEN")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "token can not be empty",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if _, err := uuid.Parse(token); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	userId, err := a.pgStore.GetVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	user, err := a.pgStore.GetUserById(ctx.Request().Context(), userId, false)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "user not found",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	user.IsActive = true

	err = a.pgStore.UpdateUser(ctx.Request().Context(), userId, user)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "error updating user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	err = a.pgStore.DeleteVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "error while deleting verify email",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	access, err := a.newWebLoginToken(userId, user.Username, "access")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting access token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	refresh, err := a.newWebLoginToken(userId, user.Username, "refresh")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting refresh token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for session",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	if err = a.pgStore.AddSession(ctx.Request().Context(), id.String(), refresh, user.Username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionId := fmt.Sprintf("%s:%s", id, userId)
	sessionCookie := a.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", access, true, time.Now().Add(time.Hour))
	refreshCookie := a.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	err = ctx.JSON(http.StatusOK, echo.Map{
		"message": "user profile activated successfully",
	})
	a.logger.Log(ctx, err)
	return err
}
