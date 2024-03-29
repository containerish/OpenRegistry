package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) VerifyEmail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	rawToken := ctx.QueryParam("token")
	if rawToken == "" {
		err := fmt.Errorf("EMPTY_TOKEN")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "token can not be empty",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	token, err := uuid.Parse(rawToken)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	userId, err := a.emailStore.GetVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	user, err := a.userStore.GetUserByID(ctx.Request().Context(), userId)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "user not found",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	user.IsActive = true

	_, err = a.userStore.UpdateUser(ctx.Request().Context(), user)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "error updating user",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = a.emailStore.DeleteVerifyEmail(ctx.Request().Context(), token)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "error while deleting verify email",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	accesssTokenOpts := &WebLoginJWTOptions{
		Id:        userId,
		Username:  user.Username,
		TokenType: AccessCookieKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}

	access, err := NewWebLoginToken(accesssTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting access token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	refreshTokenOpts := &WebLoginJWTOptions{
		Id:        userId,
		Username:  user.Username,
		TokenType: RefreshCookKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}
	refresh, err := NewWebLoginToken(refreshTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting refresh token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for session",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	if err = a.sessionStore.AddSession(ctx.Request().Context(), id, refresh, user.ID); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionId := fmt.Sprintf("%s:%s", id, userId)
	sessionCookie := a.createCookie(ctx, "session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie(ctx, AccessCookieKey, access, true, time.Now().Add(time.Hour))
	refreshCookie := a.createCookie(ctx, RefreshCookKey, refresh, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	err = ctx.JSON(http.StatusOK, echo.Map{
		"message": "user profile activated successfully",
	})
	a.logger.Log(ctx, err).Send()
	return err
}
