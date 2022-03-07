package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) SignIn(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	if err := user.Validate(); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"code":  "INVALID_CREDENTIALS",
		})
	}

	key := user.Email
	if user.Username != "" {
		key = user.Username
	}

	//bz, err := a.store.Get([]byte(key))
	userFromDb, err := a.pgStore.GetUser(ctx.Request().Context(), key)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	if !a.verifyPassword(userFromDb.Password, user.Password) {
		errMsg := fmt.Errorf("invalid password")
		a.logger.Log(ctx, errMsg)
		return ctx.JSON(http.StatusUnauthorized, errMsg)
	}

	access, refresh, err := a.newWebLoginToken(*userFromDb)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	id := uuid.NewString()
	if err = a.pgStore.AddSession(ctx.Request().Context(), id, refresh, userFromDb.Username); err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERR_CREATING_SESSION",
		})
	}
	sessionId := fmt.Sprintf("%s:%s", id, userFromDb.Id)
	sessionCookie := a.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", access, true, time.Now().Add(time.Hour))
	refreshCookie := a.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))

	a.logger.Log(ctx, err)

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)
	return ctx.JSON(http.StatusOK, echo.Map{
		"token":   access,
		"refresh": refresh,
	})
}
