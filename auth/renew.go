package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) RenewAccessToken(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	c, err := ctx.Cookie("refresh")
	if err != nil {
		if err == http.ErrNoCookie {
			echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "Unauthorised",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting refresh cookie",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	refreshCookie := c.Value
	var claims Claims
	tkn, err := jwt.ParseWithClaims(refreshCookie, &claims, func(token *jwt.Token) (interface{}, error) {
		return a.c.Registry.Auth.JWTSigningPubKey, nil
	})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "signature error, unauthorised",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing claims",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if !tkn.Valid {
		err = fmt.Errorf("invalid token, Unauthorised")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "invalid token, unauthorised",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	userId, err := uuid.Parse(claims.ID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid user id format",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	user, err := a.pgStore.GetUserByID(ctx.Request().Context(), userId)
	if err != nil {
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "user not found in database, unauthorised",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	opts := &WebLoginJWTOptions{
		Id:        userId,
		Username:  user.Username,
		TokenType: AccessCookieKey,
		Audience:  a.c.Registry.FQDN,
		Privkey:   a.c.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    a.c.Registry.Auth.JWTSigningPubKey,
	}
	tokenString, err := NewWebLoginToken(opts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating new web token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	accessCookie := a.createCookie(ctx, AccessCookieKey, tokenString, true, time.Now().Add(time.Hour))
	ctx.SetCookie(accessCookie)
	err = ctx.NoContent(http.StatusNoContent)
	a.logger.Log(ctx, err).Send()
	return err
}
