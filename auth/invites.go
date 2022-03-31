package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type List struct {
	Emails string
}

func (a *auth) Invites(ctx echo.Context) error {
	var list List
	err := json.NewDecoder(ctx.Request().Body).Decode(&list)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "error decode body, expecting and array of emails",
		})
	}
	err = a.emailClient.WelcomeEmail(strings.Split(list.Emails, ","))
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"msg":   "err sending invites",
		})
	}

	a.logger.Log(ctx, err)
	return ctx.JSON(http.StatusAccepted, echo.Map{
		"msg": "success",
	})
}
