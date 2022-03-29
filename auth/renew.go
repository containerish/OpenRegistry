package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func (a *auth) RenewAccessToken(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	c, err := ctx.Cookie("refresh")
	if err != nil {
		if err == http.ErrNoCookie {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "Unauthorised",
			})
		}
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting refresh cookie",
		})
	}
	refreshCookie := c.Value
	var claims Claims
	tkn, err := jwt.ParseWithClaims(refreshCookie, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.c.Registry.SigningSecret), nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			a.logger.Log(ctx, err)
			return ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "Signature error, unauthorised",
			})
		}
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	if !tkn.Valid {
		a.logger.Log(ctx, fmt.Errorf("invalid token, Unauthorised"))
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": "invalid token, unauthorised",
		})
	}

	userId := claims.Id
	user, err := a.pgStore.GetUserById(ctx.Request().Context(), userId)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "user not found in database, unauthorised",
		})
	}

	tokenString, err := a.newWebLoginToken(userId, user.Username, "access")
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating new web token",
		})
	}

	accessCookie := a.createCookie("access", tokenString, true, time.Now().Add(time.Hour))
	ctx.SetCookie(accessCookie)
	return ctx.NoContent(http.StatusNoContent)
}
