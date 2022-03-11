package email

import "github.com/sendgrid/sendgrid-go/helpers/mail"

func (e *email) CreateEmail(mailReq *Mail) (*mail.SGMailV3, error) {
	m := mail.NewV3Mail()

	email := mail.NewEmail(mailReq.Name, e.config.SendAs)
	m.SetFrom(email)

	m.SetTemplateID(e.config.VerifyEmailTemplate)

	p := mail.NewPersonalization()
	tos := []*mail.Email{
		mail.NewEmail(mailReq.To, mailReq.To),
	}
	p.AddTos(tos...)

	p.SetDynamicTemplateData("user", mailReq.Data.Username)
	p.SetDynamicTemplateData("link", mailReq.Data.Link)

	m.AddPersonalizations(p)
	return m, nil
}
