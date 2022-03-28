package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func (a *auth) ResetPassword(ctx echo.Context) error {
	token, ok := ctx.Get("user").(*jwt.Token)
	if !ok {
		err := fmt.Errorf("JWT token can not be empty")
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": err.Error(),
		})
	}

	c, ok := token.Claims.(*Claims)
	if !ok {
		err := fmt.Errorf("invalid claims in JWT")
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	userId := c.Id
	user, err := a.pgStore.GetUserById(ctx.Request().Context(), userId, true)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusNotFound, echo.Map{
			"error": err.Error(),
		})
	}

	var pwd *types.Password

	kind := ctx.QueryParam("kind")

	if kind == "forgot" {
		if err = a.emailClient.SendEmail(user, token.Raw, email.ResetPasswordEmailKind); err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
				"msg":   "error sending reset password link",
			})
		}
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusAccepted, echo.Map{
			"msg": "success",
		})

	}

	err = json.NewDecoder(ctx.Request().Body).Decode(&pwd)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "request body could not be decoded",
		})
	}

	if kind == "forgot_password_callback" {
		hashPassword, err := a.hashPassword(pwd.NewPassword)
		if err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		if err = a.pgStore.UpdateUserPWD(ctx.Request().Context(), userId, hashPassword); err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		return ctx.NoContent(http.StatusOK)
	}

	if !a.verifyPassword(user.Password, pwd.OldPassword) {
		err = fmt.Errorf("passwords do not match")
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	if pwd.OldPassword == pwd.NewPassword {
		err = fmt.Errorf("new password can not be same as old password")
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	newHashedPwd, err := a.hashPassword(pwd.NewPassword)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	if err = a.pgStore.UpdateUserPWD(ctx.Request().Context(), userId, newHashedPwd); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "error updating user in db",
		})
	}

	a.logger.Log(ctx, nil)
	return ctx.JSON(http.StatusAccepted, echo.Map{
		"msg": "success",
	})
}
