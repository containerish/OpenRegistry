package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/duo-labs/webauthn/webauthn"
)

func (p *pg) AddWebAuthSessionData(
	ctx context.Context,
	credentialOwnerID string,
	sessionData *webauthn.SessionData,
	sessionType string,
) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()
	_, err := p.conn.Exec(
		childCtx,
		queries.AddWebAuthNSessionData,
		credentialOwnerID,
		sessionData.UserID,
		sessionData.Challenge,
		sessionData.AllowedCredentialIDs,
		sessionData.UserVerification,
		sessionData.Extensions,
		sessionType,
	)
	if err != nil {
		return fmt.Errorf("ERR_ADD_WEB_AUTHN_SESSION_DATA :%w", err)
	}
	return nil
}

func (p *pg) GetWebAuthNSessionData(
	ctx context.Context,
	credentialOwnerID string,
	sessionType string,
) (*webauthn.SessionData, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()

	var sessionData webauthn.SessionData
	row := p.conn.QueryRow(childCtx, queries.GetWebAuthNSessionData, credentialOwnerID, sessionType)
	if err := row.Scan(
		&sessionData.UserID,
		&sessionData.Challenge,
		&sessionData.AllowedCredentialIDs,
		&sessionData.UserVerification,
		&sessionData.Extensions,
	); err != nil {
		return nil, fmt.Errorf("ERR_GET_WEB_AUTHN_SESSION_DATA: %w", err)
	}

	return &sessionData, nil
}

func (p *pg) AddWebAuthNCredentials(ctx context.Context, credentialOwnerID string, credential *webauthn.Credential) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()

	_, err := p.conn.Exec(
		childCtx,
		queries.AddWebAuthNCredentials,
		credentialOwnerID,
		credential.ID,
		credential.PublicKey,
		credential.AttestationType,
		credential.Authenticator.AAGUID,
		credential.Authenticator.SignCount,
		credential.Authenticator.CloneWarning,
	)

	if err != nil {
		return fmt.Errorf("ERR_STORE_WEB_AUTHN_SESSION_DATA: %w", err)
	}
	return nil
}

func (p *pg) GetWebAuthNCredentials(ctx context.Context, credentialOwnerID string) (*webauthn.Credential, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()

	var creds webauthn.Credential
	row := p.conn.QueryRow(childCtx, queries.GetWebAuthNCredentials, credentialOwnerID)
	err := row.Scan(
		&creds.ID,
		&creds.PublicKey,
		&creds.AttestationType,
		&creds.Authenticator.AAGUID,
		&creds.Authenticator.SignCount,
		&creds.Authenticator.CloneWarning,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_WEB_AUTHN_CREDENTIAL_DATA: %w", err)
	}
	return &creds, nil
}
