package email

import (
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/types"
)

const (
	//KindWelcomeEmail
	WelcomeEmailKind EmailKind = iota
	VerifyEmailKind
	ResetPasswordEmailKind
)

type EmailKind int8

func (e *email) SendEmail(u *types.User, token string, kind EmailKind) error {
	mailMsg, err := e.CreateEmail(u, kind, token)
	if err != nil {
		return fmt.Errorf("ERR_CREATE_EMAIL: %w", err)
	}

	resp, err := e.client.Send(mailMsg)
	if err != nil && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ERR_SEND_EMAIL: %w", err)
	}

	return nil
}
