package auth

import (
	"encoding/json"
	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (a *auth) ResetPassword(ctx echo.Context) error {

	token, ok := ctx.Get("user").(*jwt.Token)
	if !ok {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": "unauthorised, invalid token",
		})
	}

	c, ok := token.Claims.(*Claims)
	if !ok {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": "error parsing claims",
		})
	}

	userId := c.Id
	color.Magenta("userid: %s", userId)
	user, err := a.pgStore.GetUser(ctx.Request().Context(), userId)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, echo.Map{
			"error": "user not found",
		})
	}

	var pwd *types.Password

	kind := ctx.QueryParam("kind")
	if kind == "forgot" {
		return ctx.JSON(http.StatusOK, echo.Map{
			"msg": "came here",
		})

	} else if kind == "reset" {
		color.Red("came here")
		err := json.NewDecoder(ctx.Request().Body).Decode(&pwd)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": "request body could not be decoded",
			})
		}

		if !a.verifyPassword(user.Password, pwd.OldPassword) {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": "the old password and password stored in db do not match, try again",
			})
		}

		user.Password = pwd.NewPassword
		if err = a.pgStore.UpdateUser(ctx.Request().Context(), userId, user); err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": "error updating user in db",
			})
		}

	} else {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "kind must be either forgot or reset",
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}
