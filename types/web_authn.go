package types

import "github.com/duo-labs/webauthn/webauthn"

type (
	WebAuthNSessiondata struct {
		webauthn.SessionData
		CredentialOwnerId string
	}
)
