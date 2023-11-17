package email

import (
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

const (
	//KindWelcomeEmail
	WelcomeEmailKind EmailKind = iota
	VerifyEmailKind
	ResetPasswordEmailKind
)

type EmailKind int8

func (e *email) SendEmail(u *types.User, token string, kind EmailKind, webAppURL string) error {
	if e.config.Enabled {
		mailMsg, err := e.CreateEmail(u, kind, token, webAppURL)
		if err != nil {
			return fmt.Errorf("ERR_CREATE_EMAIL: %w", err)
		}

		resp, err := e.client.Send(mailMsg)
		if err != nil && resp.StatusCode != http.StatusAccepted {
			return fmt.Errorf("ERR_SEND_EMAIL: %w", err)
		}
	}

	return nil
}
