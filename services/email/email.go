package email

import (
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type email struct {
	client          *sendgrid.Client
	config          *config.Email
	backendEndpoint string
}

type MailType int

type MailData struct {
	Username string
	Link     string
}

type Mail struct {
	Data    MailData
	Name    string
	To      string
	Subject string
	Body    string
	Mtype   MailType
}

type MailService interface {
	CreateEmail(u *types.User, kind EmailKind, token string) (*mail.SGMailV3, error)
	SendEmail(u *types.User, token string, kind EmailKind) error
}

func New(config *config.Email, backendEndpoint string) MailService {
	client := sendgrid.NewSendClient(config.ApiKey)
	return &email{client: client, config: config, backendEndpoint: backendEndpoint}
}
