package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4"
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

	err = json.NewDecoder(ctx.Request().Body).Decode(&pwd)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "request body could not be decoded",
		})
	}
	_ = ctx.Request().Body.Close()

	if kind == "forgot_password_callback" {
		hashPassword, err := a.hashPassword(pwd.NewPassword)
		if err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		if user.Password == hashPassword {
			err = fmt.Errorf("new password can not be same as old password")
			a.logger.Log(ctx, err)
			// error is already user friendly
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"message": err.Error(),
			})
		}

		if err = a.pgStore.UpdateUserPWD(ctx.Request().Context(), userId, hashPassword); err != nil {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		return ctx.JSON(http.StatusAccepted, echo.Map{
			"message": "password changed successfully",
		})
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

func (a *auth) ForgotPassword(ctx echo.Context) error {
	userEmail := ctx.QueryParam("email")
	if err := a.verifyEmail(userEmail); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "email is invalid",
		})
	}

	user, err := a.pgStore.GetUser(ctx.Request().Context(), userEmail, false)
	if err != nil {
		if errors.Unwrap(err) == pgx.ErrNoRows {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "user does not exist with this email",
			})
		}

		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"message": err.Error(),
		})
	}

	if !user.IsActive {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"message": "account is inactive, please check your email and verify your account",
		})
	}

	token, err := a.newWebLoginToken(user.Id, user.Username, "service")
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_GENERATE_RESET_PASSWORD_TOKEN",
		})
	}

	if err = a.emailClient.SendEmail(user, token, email.ResetPasswordEmailKind); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error sending password reset link",
		})
	}

	a.logger.Log(ctx, err)
	return ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "a password reset link has been sent to your email",
	})

}
