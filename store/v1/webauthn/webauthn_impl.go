package webauthn

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type webauthnStore struct {
	db *bun.DB
}

func New(db *bun.DB) WebAuthnStore {
	return &webauthnStore{
		db,
	}
}

type WebAuthnStore interface {
	GetWebAuthnSessionData(ctx context.Context, userID uuid.UUID, sessionType string) (*webauthn.SessionData, error)
	GetWebAuthnCredentials(ctx context.Context, userID uuid.UUID) (*webauthn.Credential, error)
	AddWebAuthSessionData(
		ctx context.Context,
		userID uuid.UUID,
		sessionData *webauthn.SessionData,
		sessionType string,
	) error
	AddWebAuthnCredentials(ctx context.Context, userID uuid.UUID, credential *webauthn.Credential) error
	RemoveWebAuthSessionData(ctx context.Context, credentialOwnerID uuid.UUID) error
	WebauthnUserExists(ctx context.Context, email, username string) bool
}

func (ws *webauthnStore) GetWebAuthnSessionData(
	ctx context.Context,
	userId uuid.UUID,
	sessionType string,
) (*webauthn.SessionData, error) {
	session := &types.WebauthnSession{}

	err := ws.
		db.
		NewSelect().
		Model(session).
		Where("credential_owner_id = ? and session_type = ?", userId, sessionType).
		Scan(ctx)
	if err != nil {
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return &webauthn.SessionData{
		Challenge:            session.Challege,
		UserID:               session.CredentialOwnerID[:],
		AllowedCredentialIDs: session.AllowedCredentialIDs,
		Expires:              session.Expires,
		UserVerification:     session.UserVerification,
		Extensions:           session.Extensions,
	}, nil
}

func (ws *webauthnStore) GetWebAuthnCredentials(
	ctx context.Context,
	credentialOwnerId uuid.UUID,
) (*webauthn.Credential, error) {
	credential := &types.WebauthnCredential{}
	err := ws.
		db.
		NewSelect().
		Model(credential).
		Where("credential_owner_id = ?", credentialOwnerId).
		Scan(ctx)

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
	userId uuid.UUID,
	sessionData *webauthn.SessionData,
	sessionType string,
) error {
	userIDFromSession, _ := uuid.FromBytes(sessionData.UserID)

	session := &types.WebauthnSession{
		ID:                   uuid.New(),
		Expires:              sessionData.Expires,
		Extensions:           sessionData.Extensions,
		Challege:             sessionData.Challenge,
		CredentialOwnerID:    userId,
		UserVerification:     sessionData.UserVerification,
		SessionType:          sessionType,
		UserID:               userIDFromSession,
		AllowedCredentialIDs: sessionData.AllowedCredentialIDs,
	}

	if _, err := ws.db.NewInsert().Model(session).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (ws *webauthnStore) AddWebAuthnCredentials(
	ctx context.Context,
	userId uuid.UUID,
	wanCred *webauthn.Credential,
) error {
	credential := &types.WebauthnCredential{
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

func (ws *webauthnStore) RemoveWebAuthSessionData(ctx context.Context, credentialOwnerID uuid.UUID) error {
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
			"identities->'webauthn'->>'email' = ? or identities->'webauthn'->>'username' = ?",
			bun.Ident(email),
			bun.Ident(username),
		).
		Scan(ctx, &exists)
	if err != nil {
		return false
	}

	return exists
}
