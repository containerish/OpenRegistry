package postgres

import (
	"context"
	"fmt"
	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/duo-labs/webauthn/webauthn"
	"time"
)

func (p *pg) AddWebAuthSessionData(ctx context.Context, sessionData *webauthn.SessionData) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()
	_, err := p.conn.Exec(childCtx,
		queries.AddWebAuthNSessionData,
		sessionData.Challenge,
		sessionData.UserID,
		sessionData.AllowedCredentialIDs,
		sessionData.UserVerification,
		sessionData.Extensions,
	)
	if err != nil {
		return fmt.Errorf("ERR_ADD_WEB_AUTHN_SESSION_DATA :%w", err)
	}
	return nil
}

func (p *pg) GetWebAuthNSessionData(ctx context.Context, userId string) (*webauthn.SessionData, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	var sessionData webauthn.SessionData

	row := p.conn.QueryRow(childCtx, queries.GetWebAuthNSessionData, userId)
	if err := row.Scan(
		&sessionData.Challenge,
		&sessionData.UserID,
		&sessionData.AllowedCredentialIDs,
		&sessionData.UserVerification,
		&sessionData.Extensions,
	); err != nil {
		return nil, fmt.Errorf("ERR_GET_WEB_AUTHN_SESSION_DATA: %w", err)
	}

	return &sessionData, nil
}

func (p *pg) AddWebAuthNCredentials(ctx context.Context, credential *webauthn.Credential) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_, err := p.conn.Exec(
		childCtx,
		queries.AddWebAuthNCredentials,
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

func (p *pg) GetWebAuthNCredentials(ctx context.Context, id string) (*webauthn.Credential, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	var creds webauthn.Credential

	row := p.conn.QueryRow(childCtx, queries.GetWebAuthNCredentials)
	err := row.Scan(
		&creds.ID,
		&creds.PublicKey,
		&creds.AttestationType,
		&creds.Authenticator.AAGUID,
		&creds.Authenticator.SignCount,
		&creds.Authenticator.CloneWarning,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_WEB_AUTHN_SESSION_DATA: %w", err)
	}
	return &creds, nil
}
