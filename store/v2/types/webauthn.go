package types

import (
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type WebauthnSession struct {
	bun.BaseModel        `bun:"table:webauthn_session" json:"-"`
	Expires              time.Time                            `bun:"expires" json:"expires"`
	Extensions           protocol.AuthenticationExtensions    `bun:"extensions,type:jsonb" json:"extensions"`
	User                 *User                                `bun:"rel:belongs-to,join:user_id=id"`
	Challege             string                               `bun:"challenge" json:"challenge"`
	UserVerification     protocol.UserVerificationRequirement `bun:"user_verification" json:"user_verification"`
	SessionType          string                               `bun:"session_type" json:"session_type"`
	AllowedCredentialIDs [][]byte                             `bun:"allowed_credential_ids" json:"allowed_credential_ids"`
	//nolint
	CredentialOwnerID uuid.UUID `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	UserID            uuid.UUID `bun:"user_id,type:uuid" json:"user_id"`
}

type WebauthnCredential struct {
	bun.BaseModel     `bun:"table:webauthn_credentials" json:"-"`
	User              *User                             `bun:"rel:belongs-to,join:credential_owner_id=id"`
	Authenticator     webauthn.Authenticator            `bun:"authenticator" json:"authenticator"`
	AttestationType   string                            `bun:"attestation_type" json:"attestation_type"`
	ID                []byte                            `bun:"id" json:"id"`
	PublicKey         []byte                            `bun:"public_key" json:"public_key"`
	Transport         []protocol.AuthenticatorTransport `bun:"transport" json:"transport"`
	CredentialOwnerID uuid.UUID                         `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	Flags             webauthn.CredentialFlags          `bun:"flags" json:"flags"`
}
