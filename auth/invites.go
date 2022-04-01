package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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

	if list.Emails == "" {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "cannot send empty list",
		})
	}

	emails := strings.Split(list.Emails, ",")

	if err = a.validateEmailList(emails); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	err = a.emailClient.WelcomeEmail(emails)
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

func (a *auth) validateEmailList(emails []string) error {
	for _, email := range emails {
		if err := a.verifyEmail(email); err != nil {
			return fmt.Errorf("ERR_INVALID_EMAIL: %s", email)
		}
	}

	return nil
}

func (a *auth) verifyEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email can not be empty")
	}

	emailReg := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}" +
		"[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	if !emailReg.Match([]byte(email)) {
		return fmt.Errorf("email format invalid")
	}

	return nil
}
