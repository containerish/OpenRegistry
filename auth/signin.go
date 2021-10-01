package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func (a *auth) SignIn(ctx echo.Context) error {
	var user User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}
	if user.Email == "" || user.Password == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "Email/Password cannot be empty",
		})
	}

	if err := verifyEmail(user.Email); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	key := fmt.Sprintf("%s/%s", UserNameSpace, user.Username)
	bz, err := a.store.Get([]byte(key))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	var userFromDb User
	if err := json.Unmarshal(bz, &userFromDb); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	if !a.verifyPassword(userFromDb.Password, user.Password) {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": "invalid password",
		})
	}

	tokenLife := time.Now().Add(time.Hour * 24 * 14).Unix()
	token, err := a.newToken(user, tokenLife)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"token":      token,
		"expires_in": tokenLife,
		"issued_at":  time.Now().Unix(),
	})

}
