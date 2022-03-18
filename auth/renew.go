package auth

import (
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"net/http"
	"time"
)

func (a *auth) RenewAccessToken(ctx echo.Context) error {

	c, err := ctx.Cookie("refresh")
	if err != nil {
		if err == http.ErrNoCookie {
			return ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "Unauthorised",
			})
		}
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
			return ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "Signature error, unauthorised",
			})
		}
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}
	if !tkn.Valid {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "invalid token, unauthorised",
		})
	}

	userId := claims.Id
	user, err := a.pgStore.GetUserById(ctx.Request().Context(), userId)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "user not found in database, unauthorised",
		})
	}

	tokenString, err := a.newWebLoginToken(userId, user.Username, "access")
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating new web token",
		})
	}

	accessCookie := a.createCookie("access", tokenString, true, time.Now().Add(time.Hour))
	ctx.SetCookie(accessCookie)
	return ctx.NoContent(http.StatusNoContent)
}
