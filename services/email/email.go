package email

import (
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type email struct {
	client *sendgrid.Client
	config *config.Email
}

type MailType int

type MailData struct {
	Username string
	Link     string
}

type Mail struct {
	Data    MailData
	Name    string
	Subject string
	Body    string
	To      []string
	Mtype   MailType
}

type MailService interface {
	CreateEmail(u *types.User, kind EmailKind, token string, webAppURL string) (*mail.SGMailV3, error)
	SendEmail(u *types.User, token string, kind EmailKind, webAppURL string) error
	WelcomeEmail(list []string, webAppURL string) error
}

func New(config *config.Email) MailService {
	if config.Enabled {
		client := sendgrid.NewSendClient(config.ApiKey)
		return &email{client: client, config: config}
	}

	return nil
}
