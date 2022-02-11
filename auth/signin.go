package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

func (a *auth) SignIn(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
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

	accessCookie := &http.Cookie{
		Name:    "access",
		Value:   access,
		Expires: time.Now().Add(time.Hour),
		Path:    "/",
	}

	refreshCookie := &http.Cookie{
		Name:    "refresh",
		Value:   refresh,
		Expires: time.Now().Add(time.Hour * 750),
		Path:    "/",
	}
	http.SetCookie(ctx.Response(), accessCookie)
	http.SetCookie(ctx.Response(), refreshCookie)
	a.logger.Log(ctx, err)

	return ctx.JSON(http.StatusOK, echo.Map{
		"token":      token,
		"expires_in": tokenLife,
		"issued_at":  time.Now().Unix(),
	})
}
