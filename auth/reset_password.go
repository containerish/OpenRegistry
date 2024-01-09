package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (a *auth) ResetForgottenPassword(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	token, ok := ctx.Get("user").(*jwt.Token)
	if !ok {
		err := fmt.Errorf("ERR_EMPTY_TOKEN")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "JWT token can not be empty",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	c, ok := token.Claims.(*Claims)
	if !ok {
		err := fmt.Errorf("ERR_INVALID_CLAIMS")
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "invalid claims in JWT",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	var pwd *types.ResetPasswordRequest
	if err := json.NewDecoder(ctx.Request().Body).Decode(&pwd); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "request body could not be decoded",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	userId, err := uuid.Parse(c.ID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid user id format",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	user, err := a.userStore.GetUserByID(ctx.Request().Context(), userId)
	if err != nil {
		echoErr := ctx.JSON(http.StatusNotFound, echo.Map{
			"error":   err.Error(),
			"message": "error getting user by ID from DB",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = types.ValidatePassword(pwd.NewPassword); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"message": `password must be alphanumeric, at least 8 chars long, must have at least one special character
and an uppercase letter`,
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if a.verifyPassword(user.Password, pwd.NewPassword) {
		err = fmt.Errorf("new password can not be same as old password")
		// error is already user friendly
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	hashPassword, err := a.hashPassword(pwd.NewPassword)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_HASH_NEW_PASSWORD",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = a.userStore.UpdateUserPWD(ctx.Request().Context(), userId, hashPassword); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error updating new password",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "password changed successfully",
	})
	a.logger.Log(ctx, err).Send()
	return err
}

func (a *auth) ResetPassword(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		err := fmt.Errorf("unauthorized: missing user auth credentials")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	var pwd types.ResetPasswordRequest
	err := json.NewDecoder(ctx.Request().Body).Decode(&pwd)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "request body could not be decoded",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	// compare the current password with password hash from DB
	if !a.verifyPassword(user.Password, pwd.OldPassword) {
		err = fmt.Errorf("ERR_WRONG_PASSWORD")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "password is wrong",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	hashPassword, err := a.hashPassword(pwd.NewPassword)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_HASH_NEW_PASSWORD",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if user.Password == hashPassword {
		err = fmt.Errorf("new password can not be same as old password")
		// error is already user friendly
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = types.ValidatePassword(pwd.NewPassword); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"message": `password must be alphanumeric, at least 8 chars long, must have at least one special character
and an uppercase letter`,
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = a.userStore.UpdateUserPWD(ctx.Request().Context(), user.ID, hashPassword); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error updating new password",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "password changed successfully",
	})
	a.logger.Log(ctx, err).Send()
	return err
}

func (a *auth) ForgotPassword(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	userEmail := ctx.QueryParam("email")
	if err := a.verifyEmail(userEmail); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "email is invalid",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	user, err := a.userStore.GetUserByEmail(ctx.Request().Context(), userEmail)
	if err != nil {
		if errors.Unwrap(err) == pgx.ErrNoRows {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "user does not exist with this email",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error get user from DB with this email",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if !user.IsActive {
		err = fmt.Errorf("account is inactive, please check your email and verify your account")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"message": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	opts := &WebLoginJWTOptions{
		Id:        user.ID,
		Username:  user.Username,
		TokenType: AccessCookieKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}
	token, err := NewWebLoginToken(opts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error generating reset password token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	webAppURL := a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment)

	if err = a.emailClient.SendEmail(user, token, email.ResetPasswordEmailKind, webAppURL); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error sending password reset link",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "a password reset link has been sent to your email",
	})
	a.logger.Log(ctx, err).Send()
	return err
}
