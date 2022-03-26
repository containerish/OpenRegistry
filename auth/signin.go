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

	var user User
	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}
	if user.Email == "" && user.Username == "" {
		errMsg := fmt.Errorf("email and username cannot be empty, please provide at least one of them")
		a.logger.Log(ctx, errMsg)
		return ctx.JSON(http.StatusBadRequest, errMsg)
	}

	if user.Password == "" {
		errMsg := fmt.Errorf("password cannot be empty")
		a.logger.Log(ctx, errMsg)
		return ctx.JSON(http.StatusBadRequest, errMsg)
	}

	var key string

	if user.Email != "" {
		if err := verifyEmail(user.Email); err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		key = user.Email
	} else {
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

	tokenLife := time.Now().Add(time.Hour * 24 * 14).Unix()
	uu := User{
		Email:    userFromDb.Email,
		Username: userFromDb.Username,
	}

	token, err := a.newToken(uu, tokenLife)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	a.logger.Log(ctx, nil)
	return ctx.JSON(http.StatusOK, echo.Map{
		"token":      token,
		"expires_in": tokenLife,
		"issued_at":  time.Now().Unix(),
	})

}
