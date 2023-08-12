package types

import (
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/uptrace/bun"
)

type WebauthnSession struct {
	bun.BaseModel `bun:"table:webauthn_session" json:"-"`

	Expires    time.Time                         `bun:"expires" json:"expires"`
	Extensions protocol.AuthenticationExtensions `bun:"extensions,type:jsonb" json:"extensions"`
	User       *User                             `bun:"rel:belongs-to,join:user_id=id"`
	Challege   string                            `bun:"challenge" json:"challenge"`
	//nolint
	CredentialOwnerID string                               `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	UserVerification  protocol.UserVerificationRequirement `bun:"user_verification" json:"user_verification"`
	SessionType       string                               `bun:"session_type" json:"session_type"`
	UserID            []byte                               `bun:"user_id" json:"user_id"`
	//nolint
	AllowedCredentialIDs [][]byte `bun:"allowed_credential_ids" json:"allowed_credential_ids"`
}

type WebauthnCredential struct {
	bun.BaseModel `bun:"table:webauthn_credentials" json:"-"`

	Authenticator     webauthn.Authenticator            `bun:"authenticator" json:"authenticator"`
	CredentialOwnerID string                            `bun:"credential_owner_id,type:uuid" json:"credential_owner_id"`
	User              *User                             `bun:"rel:belongs-to,join:credential_owner_id=id"`
	AttestationType   string                            `bun:"attestation_type" json:"attestation_type"`
	ID                []byte                            `bun:"id" json:"id"`
	PublicKey         []byte                            `bun:"public_key" json:"public_key"`
	Transport         []protocol.AuthenticatorTransport `bun:"transport" json:"transport"`
	Flags             webauthn.CredentialFlags          `bun:"flags" json:"flags"`
	// CloneWarning      bool                              `bun:"clone_warning" json:"clone_warning"`
	// SignCount         uint64                            `bun:"sign_count" json:"sign_count"`
	// AAGUID            []byte                            `bun:"aaguid" json:"aaguid"`
}
