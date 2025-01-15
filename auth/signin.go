package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (a *auth) SignIn(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid JSON object",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	if user.Username == types.SystemUsernameIPFS {
		err := fmt.Errorf("login is not allowed for this user")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err := user.Validate(true)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   "invalid data provided for user login",
			"message": err.Error(),
			"code":    "INVALID_CREDENTIALS",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	var userFromDb *types.User
	if user.Email != "" {
		userFromDb, err = a.userStore.GetUserByEmail(ctx.Request().Context(), user.Email)
	} else {
		userFromDb, err = a.userStore.GetUserByUsername(ctx.Request().Context(), user.Username)
	}

	if err != nil {
		if errors.Unwrap(err) == pgx.ErrNoRows {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "user not found",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error, failed to get user",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if !userFromDb.IsActive {
		err = fmt.Errorf("account is inactive, please check your email and verify your account")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   "ERR_USER_INACTIVE",
			"message": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if userFromDb.Password == "" {
		if userFromDb.Identities.GetGitHubIdentity() != nil {
			errMsg := fmt.Errorf("login method is not available for this user. Please try GitHub method")
			echoErr := ctx.JSON(http.StatusPreconditionFailed, echo.Map{
				"message": errMsg.Error(),
				"error":   "LOGIN_METHOD_CONFLICT",
			})
			a.logger.Log(ctx, errMsg).Send()
			return echoErr
		}

		if userFromDb.Identities.GetWebauthnIdentity() != nil {
			errMsg := fmt.Errorf("login method is not available for this user. Please try the Webauthn method")
			echoErr := ctx.JSON(http.StatusPreconditionFailed, echo.Map{
				"message": errMsg.Error(),
				"error":   "LOGIN_METHOD_CONFLICT",
			})
			a.logger.Log(ctx, errMsg).Send()
			return echoErr
		}
	}

	if !a.verifyPassword(userFromDb.Password, user.Password) {
		err = fmt.Errorf("password is incorrect")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   "ERR_INCORRECT_PASSWORD",
			"message": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	accessTokenOpts := &WebLoginJWTOptions{
		Id:        userFromDb.ID,
		Username:  userFromDb.Username,
		TokenType: AccessCookieKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}
	access, err := NewWebLoginToken(accessTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating web login token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	refreshTokenOpts := &WebLoginJWTOptions{
		Id:        userFromDb.ID,
		Username:  userFromDb.Username,
		TokenType: RefreshCookKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}

	refresh, err := NewWebLoginToken(refreshTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating refresh token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating session id",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	if err = a.sessionStore.AddSession(ctx.Request().Context(), id, refresh, userFromDb.ID); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	sessionId := fmt.Sprintf("%s:%s", id, userFromDb.ID)
	sessionCookie := a.createCookie(ctx, SessionCookieKey, sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie(ctx, AccessCookieKey, access, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie(ctx, RefreshCookKey, refresh, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)
	err = ctx.JSON(http.StatusOK, echo.Map{
		SessionCookieKey: sessionId,
		AccessCookieKey:  access,
		RefreshCookKey:   refresh,
	})
	a.logger.Log(ctx, err).Send()
	return err
}
