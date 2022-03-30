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
	Name    string
	To      string
	Subject string
	Body    string
	Mtype   MailType
	Data    MailData
}

type MailService interface {
	CreateEmail(mailReq *Mail) (*mail.SGMailV3, error)
	SendEmail(u *types.User, token string) error
}

func New(config *config.Email, backendEndpoint string) MailService {
	client := sendgrid.NewSendClient(config.ApiKey)
	return &email{client: client, config: config, backendEndpoint: backendEndpoint}
}
