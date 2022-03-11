package auth

import (
	"encoding/base64"
	"fmt"
	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"net/http"
	"time"
)

func (a *auth) VerifyEmail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	token := ctx.QueryParam("token")
	if token == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "EMPTY_TOKEN",
		})
	}

	jToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "EMPTY_TOKEN",
		})
	}

	var t *jwt.Token

	_, err = jwt.Parse(string(jToken), func(jt *jwt.Token) (interface{}, error) {
		if jt == nil {
			return nil, fmt.Errorf("ERR_PARSE_JWT_TOKEN")
		}
		t = jt
		return nil, nil
	})

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "ERR_CONVERT_CLAIMS",
		})
	}

	id := claims["ID"].(string)
	tokenFromDb, err := a.pgStore.GetVerifyEmail(ctx.Request().Context(), id)
	if tokenFromDb != string(jToken) {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "ERR_TOKEN_MISMATCH",
		})
	}

	user, err := a.pgStore.GetUserById(ctx.Request().Context(), id)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "USER_NOT_FOUND",
		})
	}

	user.IsActive = true

	err = a.pgStore.UpdateUser(ctx.Request().Context(), id, user)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "ERROR_UPDATE_USER",
		})
	}

	err = a.pgStore.DeleteVerifyEmail(ctx.Request().Context(), id)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "ERROR_DELETE_VERIFY_EMAIL",
		})
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"message": "success",
	})
}
