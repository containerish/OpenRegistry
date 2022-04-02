package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

func (a *auth) SignIn(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid JSON object",
		})
	}

	if err := user.Validate(); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid data provided for user login",
			"code":    "INVALID_CREDENTIALS",
		})
	}

	key := user.Email
	if user.Username != "" {
		key = user.Username
	}

	userFromDb, err := a.pgStore.GetUser(ctx.Request().Context(), key, true)
	if err != nil {
		a.logger.Log(ctx, err)

		if errors.Unwrap(err) == pgx.ErrNoRows {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "user not found",
			})
		}

		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"message": err.Error(),
		})
	}

	if !userFromDb.IsActive {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"message": "account is inactive, please check your email and verify your account",
		})
	}

	if !a.verifyPassword(userFromDb.Password, user.Password) {
		err = fmt.Errorf("password is wrong")
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"message": err.Error(),
		})
	}

	access, err := a.newWebLoginToken(userFromDb.Id, userFromDb.Username, "access")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_WEB_LOGIN_TOKEN",
		})
	}

	refresh, err := a.newWebLoginToken(userFromDb.Id, userFromDb.Username, "refresh")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_WEB_LOGIN_REFRESH_TOKEN",
		})
	}

	id := uuid.NewString()
	if err = a.pgStore.AddSession(ctx.Request().Context(), id, refresh, userFromDb.Username); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERR_CREATING_SESSION",
		})
	}

	sessionId := fmt.Sprintf("%s:%s", id, userFromDb.Id)
	sessionCookie := a.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", access, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)
	err = ctx.JSON(http.StatusOK, echo.Map{
		"token":   access,
		"refresh": refresh,
	})
	a.logger.Log(ctx, err)
	return err
}
