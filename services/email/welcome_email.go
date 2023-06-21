package email

import (
	"fmt"
	"net/http"

	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func (e *email) WelcomeEmail(list []string, baseURL string) error {
	if e.config.Enabled {
		mailReq := &Mail{}
		m := mail.NewV3Mail()

		m.SetTemplateID(e.config.WelcomeEmailTemplateId)
		mailReq.Subject = "Welcome to OpenRegistry"
		mailReq.Data.Link = fmt.Sprintf("%s/send-email/welcome", baseURL)

		email := mail.NewEmail("Team OpenRegistry", e.config.SendAs)
		m.SetFrom(email)
		p := mail.NewPersonalization()

		var tos []*mail.Email
		for _, v := range list {
			tos = append(tos, mail.NewEmail(v, v))
		}

		p.AddTos(tos...)
		m.AddPersonalizations(p)
		p.Subject = mailReq.Subject

		resp, err := e.client.Send(m)
		if err != nil && resp.StatusCode != http.StatusAccepted {
			return fmt.Errorf("ERR_SEND_EMAIL: %w", err)
		}
	}

	return nil
}
