package email

import (
	"fmt"

	"github.com/containerish/OpenRegistry/types"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

func (e *email) CreateEmail(u *types.User, kind EmailKind, token string) (*mail.SGMailV3, error) {
	mailReq := &Mail{}
	m := mail.NewV3Mail()

	mailReq.To = append(mailReq.To, u.Email)
	mailReq.Data.Username = u.Username

	name := u.Username
	if u.Name != "" {
		name = u.Name
	}

	switch kind {
	case VerifyEmailKind:
		m.SetTemplateID(e.config.VerifyEmailTemplateId)
		mailReq.Name = name
		mailReq.Subject = "Verify Email"
		mailReq.Data.Link = fmt.Sprintf("%s/auth/verify?token=%s", e.baseURL, token)

	case ResetPasswordEmailKind:
		m.SetTemplateID(e.config.ForgotPasswordTemplateId)
		mailReq.Name = name
		mailReq.Subject = "Forgot Password"
		mailReq.Data.Link = fmt.Sprintf("%s/auth/forgot-password?token=%s", e.baseURL, token)

	default:
		return nil, fmt.Errorf("incorrect email kind")
	}

	email := mail.NewEmail(mailReq.Name, e.config.SendAs)
	m.SetFrom(email)
	p := mail.NewPersonalization()

	tos := []*mail.Email{
		mail.NewEmail(mailReq.Name, mailReq.To[0]),
	}

	p.AddTos(tos...)

	p.SetDynamicTemplateData("user", mailReq.Data.Username)
	p.SetDynamicTemplateData("link", mailReq.Data.Link)

	m.AddPersonalizations(p)
	return m, nil
}
