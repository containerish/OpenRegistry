package email

import (
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type email struct {
	client  *sendgrid.Client
	config  *config.Email
	baseURL string
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
	CreateEmail(u *types.User, kind EmailKind, token string) (*mail.SGMailV3, error)
	SendEmail(u *types.User, token string, kind EmailKind) error
	WelcomeEmail(list []string) error
}

func New(config *config.Email, baseURL string) MailService {
	client := sendgrid.NewSendClient(config.ApiKey)
	return &email{client: client, config: config, baseURL: baseURL}
}
