package types

import (
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type (
	WebAuthNSessiondata struct {
		webauthn.SessionData
		CredentialOwnerId string
	}
)

// WebAuthnID - User ID according to the Relying Party
func (u *User) WebAuthnID() []byte {
	// TODO(jay-dee7): This will panic
	userID := uuid.MustParse(u.Id)
	return userID[:]
}

// WebAuthnName - User Name according to the Relying Party
func (u *User) WebAuthnName() string {
	return u.Username
}

// WebAuthnDisplayName - Display Name of the user
func (u *User) WebAuthnDisplayName() string {
	return u.Username
}

// WebAuthnIcon - User's icon url
func (u *User) WebAuthnIcon() string {
	return u.AvatarURL
}

// WebAuthnCredentials - Credentials owned by the user
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (u *User) AddWebAuthNCredential(creds *webauthn.Credential) {
	u.credentials = append(u.credentials, *creds)
}

func (u *User) AddWebAuthNCredentials(creds ...*webauthn.Credential) {
	for _, c := range creds {
		if c == nil {
			continue
		}

		u.credentials = append(u.credentials, *c)
	}
}

func (u *User) GetExistingPublicKeyCredentials() []protocol.CredentialDescriptor {
	var list []protocol.CredentialDescriptor

	for _, cred := range u.credentials {
		list = append(list, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: cred.ID,
		})
	}

	return list
}
