package email

import (
	"fmt"
	"net/http"

	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func (e *email) WelcomeEmail(list []string) error {

	mailReq := &Mail{}
	m := mail.NewV3Mail()

	m.SetTemplateID(e.config.WelcomeEmailTemplateId)
	mailReq.Name = "OpenRegistry"
	mailReq.Subject = "Welcome to OpenRegistry"
	mailReq.Data.Link = fmt.Sprintf("%s/send-email/welcome", e.backendEndpoint)

	email := mail.NewEmail(mailReq.Name, e.config.SendAs)
	m.SetFrom(email)
	p := mail.NewPersonalization()

	var tos []*mail.Email
	for _, v := range list {
		tos = append(tos, mail.NewEmail(v, v))
	}
	p.AddTos(tos...)
	m.AddPersonalizations(p)

	resp, err := e.client.Send(m)
	if err != nil && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ERR_SEND_EMAIL: %w", err)
	}
	return nil
}
