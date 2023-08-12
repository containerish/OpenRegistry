package webauthn

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/uptrace/bun"
)

type webauthnStore struct {
	db *bun.DB
}

func NewStore(db *bun.DB) WebAuthnStore {
	return &webauthnStore{
		db,
	}
}

type WebAuthnStore interface {
	GetWebAuthnSessionData(ctx context.Context, userId string, sessionType string) (*webauthn.SessionData, error)
	GetWebAuthnCredentials(ctx context.Context, userId string) (*webauthn.Credential, error)
	AddWebAuthSessionData(
		ctx context.Context,
		userId string,
		sessionData *webauthn.SessionData,
		sessionType string,
	) error
	AddWebAuthnCredentials(ctx context.Context, userId string, credential *webauthn.Credential) error
	RemoveWebAuthSessionData(ctx context.Context, credentialOwnerID string) error
	WebauthnUserExists(ctx context.Context, email, username string) bool
}

func (ws *webauthnStore) GetWebAuthnSessionData(
	ctx context.Context,
	userId string,
	sessionType string,
) (*webauthn.SessionData, error) {
	session := &types.WebauthnSession{}

	_, err := ws.
		db.
		NewSelect().
		Model(session).
		Where("credential_owner_id = ?1 and session_type = ?2", bun.Ident(userId), bun.Ident(sessionType)).
		Exec(ctx)
	if err != nil {
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return &webauthn.SessionData{
		Challenge:            session.Challege,
		UserID:               session.UserID,
		AllowedCredentialIDs: session.AllowedCredentialIDs,
		Expires:              session.Expires,
		UserVerification:     session.UserVerification,
		Extensions:           session.Extensions,
	}, nil
}

func (ws *webauthnStore) GetWebAuthnCredentials(
	ctx context.Context,
	credentialOwnerId string,
) (*webauthn.Credential, error) {
	credential := &types.WebauthnCredential{}
	_, err := ws.
		db.
		NewSelect().
		Model(credential).
		Where("credential_owner_id = ?", credentialOwnerId).
		Exec(ctx)

	if err != nil {
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return &webauthn.Credential{
		ID:              credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       credential.Transport,
		Flags:           credential.Flags,
		Authenticator:   credential.Authenticator,
	}, nil
}

func (ws *webauthnStore) AddWebAuthSessionData(
	ctx context.Context,
	userId string,
	sessionData *webauthn.SessionData,
	sessionType string,
) error {
	session := &types.WebauthnSession{
		Expires:              sessionData.Expires,
		Extensions:           sessionData.Extensions,
		Challege:             sessionData.Challenge,
		CredentialOwnerID:    userId,
		UserVerification:     sessionData.UserVerification,
		SessionType:          sessionType,
		UserID:               sessionData.UserID,
		AllowedCredentialIDs: sessionData.AllowedCredentialIDs,
	}

	if _, err := ws.db.NewInsert().Model(session).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (ws *webauthnStore) AddWebAuthnCredentials(
	ctx context.Context,
	userId string,
	wanCred *webauthn.Credential,
) error {
	credential := types.WebauthnCredential{
		Authenticator:     wanCred.Authenticator,
		CredentialOwnerID: userId,
		AttestationType:   wanCred.AttestationType,
		ID:                wanCred.ID,
		PublicKey:         wanCred.PublicKey,
		Transport:         wanCred.Transport,
		Flags:             wanCred.Flags,
	}

	if _, err := ws.db.NewInsert().Model(credential).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (ws *webauthnStore) RemoveWebAuthSessionData(ctx context.Context, credentialOwnerID string) error {
	_, err := ws.
		db.
		NewDelete().
		Model(&types.WebauthnSession{}).
		Where("credential_owner_id = ?", credentialOwnerID).
		Exec(ctx)

	if err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

func (ws *webauthnStore) WebauthnUserExists(ctx context.Context, email, username string) bool {
	var exists bool
	err := ws.
		db.
		NewSelect().
		Model(&types.User{}).
		Where(
			"identities->'webauthn'->>'email' = ?1 or identities->'webauthn'->>'username' = ?",
			bun.Ident(email),
			bun.Ident(username),
		).
		Scan(ctx, &exists)
	if err != nil {
		return false
	}

	return exists
}
