package types

import (
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type WebauthnSession struct {
	bun.BaseModel `bun:"table:webauthn_session" json:"-"`

	Expires          time.Time                            `bun:"expires" json:"expires"`
	Extensions       protocol.AuthenticationExtensions    `bun:"extensions,type:jsonb" json:"extensions"`
	User             *User                                `bun:"rel:belongs-to,join:user_id=id"`
	Challege         string                               `bun:"challenge" json:"challenge"`
	UserVerification protocol.UserVerificationRequirement `bun:"user_verification" json:"user_verification"`
	SessionType      string                               `bun:"session_type" json:"session_type"`
	//nolint
	AllowedCredentialIDs [][]byte  `bun:"allowed_credential_ids,type:bytea" json:"allowed_credential_ids"`
	ID                   uuid.UUID `bun:"id,pk,type:uuid" json:"id"`
	CredentialOwnerID    uuid.UUID `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	UserID               uuid.UUID `bun:"user_id,type:uuid" json:"user_id"`
}

type WebauthnCredential struct {
	bun.BaseModel `bun:"table:webauthn_credentials" json:"-"`

	User              *User                             `bun:"rel:belongs-to,join:credential_owner_id=id"`
	Authenticator     webauthn.Authenticator            `bun:"authenticator" json:"authenticator"`
	AttestationType   string                            `bun:"attestation_type" json:"attestation_type"`
	ID                []byte                            `bun:"id,pk,type:bytea" json:"id"`
	PublicKey         []byte                            `bun:"public_key,type:bytea," json:"public_key"`
	Transport         []protocol.AuthenticatorTransport `bun:"transport" json:"transport"`
	CredentialOwnerID uuid.UUID                         `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	Flags             webauthn.CredentialFlags          `bun:"flags" json:"flags"`
}
