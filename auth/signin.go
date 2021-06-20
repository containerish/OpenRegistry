package auth

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (a *auth) SignIn(ctx echo.Context) error {
	var user User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err!= nil {
		return ctx.JSON(http.StatusBadRequest,echo.Map{
			"error": err.Error(),
		})
	}
	if user.Email == "" || user.Password == ""{
		return ctx.JSON(http.StatusBadRequest,echo.Map{
			"error": "Email/Password cannot be empty",
		})
	}

	if err:= verifyEmail(user.Email); err!= nil {
		return ctx.JSON(http.StatusBadRequest,echo.Map{
			"error": err.Error(),
		})
	}

	key := fmt.Sprintf("%s/%s",UserNameSpace,user.Email)
	bz,err := a.store.Get([]byte(key))
	if err!= nil{
		return ctx.JSON(http.StatusBadRequest,echo.Map{
			"error": err.Error(),
		})
	}

	var userFromDb User
	if err := json.Unmarshal(bz,&userFromDb); err!= nil {
		return ctx.JSON(http.StatusInternalServerError,echo.Map{
			"error": err.Error(),
		})
	}

	if !a.verifyPassword(userFromDb.Password,user.Password) {
		return ctx.JSON(http.StatusUnauthorized,echo.Map{
			"error": "invalid password",
		})
	}

	token,err := a.newToken(user)
	if err!= nil{
	    return ctx.JSON(http.StatusInternalServerError,echo.Map{
	    	"error": err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK,echo.Map{
		"message": "user authenticated",
		"token": token,
	})

}
