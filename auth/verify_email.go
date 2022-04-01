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
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "token can not be empty",
		})
	}

	if _, err := uuid.Parse(token); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERR_PARSE_TOKEN",
		})
	}

	userId, err := a.pgStore.GetVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid token",
		})
	}

	user, err := a.pgStore.GetUserById(ctx.Request().Context(), userId, false)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "USER_NOT_FOUND",
		})
	}

	user.IsActive = true

	err = a.pgStore.UpdateUser(ctx.Request().Context(), userId, user)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "ERROR_UPDATE_USER",
		})
	}

	err = a.pgStore.DeleteVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "ERROR_DELETE_VERIFY_EMAIL",
		})
	}

	access, err := a.newWebLoginToken(userId, user.Username, "access")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	refresh, err := a.newWebLoginToken(userId, user.Username, "refresh")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	id := uuid.NewString()
	if err = a.pgStore.AddSession(ctx.Request().Context(), id, refresh, user.Username); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERR_CREATING_SESSION",
		})
	}

	sessionId := fmt.Sprintf("%s:%s", id, userId)
	sessionCookie := a.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", access, true, time.Now().Add(time.Hour))
	refreshCookie := a.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))

	a.logger.Log(ctx, err)
	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	return ctx.JSON(http.StatusOK, echo.Map{
		"message": "user profile activated successfully",
	})
}
