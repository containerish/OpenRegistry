package email

import (
	"encoding/base64"
	"fmt"
	"github.com/containerish/OpenRegistry/types"
	"net/http"
)

func (e *email) SendEmail(u *types.User, token string) error {
	token = base64.StdEncoding.EncodeToString([]byte(token))
	mailReq := &Mail{
		Name:    "Verify OpenRegistry Signup",
		To:      u.Email,
		Subject: "OpenRegistry - Signup Verification Email",
		Data: MailData{
			Username: u.Username,
			Link:     fmt.Sprintf("%s/auth/signup/verify?token=%s", e.backendEndpoint, token),
		},
	}

	mailMsg, err := e.CreateEmail(mailReq)
	if err != nil {
		return fmt.Errorf("ERR_CREATE_EMAIL: %w", err)
	}

	resp, err := e.client.Send(mailMsg)
	if err != nil && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ERR_SEND_EMAIL: %w", err)
	}

	return nil
}
