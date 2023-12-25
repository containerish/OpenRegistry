package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

type List struct {
	Emails []string `json:"emails"`
}

const MaxAllowedInvites = 5

func (a *auth) Invites(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var body List
	err := ctx.Bind(&body)
	// err := json.NewDecoder(ctx.Request().Body).Decode(&list)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error decode body, expecting an array of emails",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	if len(body.Emails) == 0 {
		err = fmt.Errorf("ERR_EMPTY_LIST")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err,
			"message": "cannot send empty list",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if len(body.Emails) > MaxAllowedInvites {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "TOO_MANY_INVITES",
			"message": fmt.Sprintf(
				"the request includes %d invites but maximum allowed invites are %d",
				len(body.Emails),
				MaxAllowedInvites,
			),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = a.validateEmailList(body.Emails); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error validating email list",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	webAppURL := a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment)
	err = a.emailClient.WelcomeEmail(body.Emails, webAppURL)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "err sending invites",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.JSON(http.StatusAccepted, echo.Map{
		"message": "invites sent successfully",
	})
	a.logger.Log(ctx, err).Send()
	return err
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

	if !config.EmailRegex.Match([]byte(email)) {
		return fmt.Errorf("email format invalid")
	}

	return nil
}
