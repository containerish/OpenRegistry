package webauthn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/types"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v4"
)

type (
	WebAuthnService interface {
		// BeginRegistration takes a WebAuthnUser and performs the "Server" logic on it. The actual work is done by
		// the underlying webauthn library "github.com/go-webauthn/webauthn" but normal sanity checks are performed
		// here like Only perform the "BeginRegistration" flow is the user doesn't already exist
		BeginRegistration(ctx context.Context, user *WebAuthnUser) (*protocol.CredentialCreation, error)

		// FinishRegistration works like sort of a commit txn in database but in Webautnn context.
		// A user must perform a BeginRegistration step before proceeding with this.
		// Also, user is responsible for handling the failed and successful states for this, i.e, This method does not
		// commit rollback your changes into the database. It only takes care of WebAuthn stuff
		FinishRegistration(ctx context.Context, opts *FinishRegistrationOpts) error

		BeginLogin(ctx context.Context, opts *BeginLoginOptions) (*protocol.CredentialAssertion, error)
		FinishLogin(ctx context.Context, opts *FinishLoginOpts) error

		// RollbackRegisteration rolls a registration back. This can be specially useful for scenarios like when the
		// user does not provide input to the authentication
		RollbackRegisteration(ctx context.Context, username string) error
	}

	webAuthnService struct {
		cfg      *config.WebAuthnConfig
		store    postgres.WebAuthN
		txnStore map[string]*webAuthNMeta
		core     *webauthn.WebAuthn
	}

	webAuthNMeta struct {
		expiresAt time.Time
		txn       pgx.Tx
	}
)

// Inspired from https://github.com/passwordless-id/webauthn#how-does-it-work
// More of a permalink: https://camo.githubusercontent.com/56fd16123e9cef7d5ed6994812d0edef43e13c2f4bae12a0f7e06b6b9760fd57/68747470733a2f2f70617373776f72646c6573732e69642f70726f746f636f6c732f776562617574686e2f6f766572766965772e737667
//
//		    ┌────────┐                              ┌─────────┐                              ┌────────┐
//			│  User  │                              │ Browser │                              │ Server │
//		    └────────┘                              └─────────┘                              └────────┘
//				┃                                        ┃                                       ┃
//				┃                                        ┃          ┌─────────────────┐          ┃
//	   ─────────┃────────────────────────────────────────┃──────────│                 │──────────┃──────────────
//	   ─────────┃────────────────────────────────────────┃──────────│ Authentication  │──────────┃──────────────
//	   ─────────┃────────────────────────────────────────┃──────────│                 │──────────┃──────────────
//		        ┃                                        ┃          └─────────────────┘          ┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃           I want to register          ┃
//		        ┃                                        ┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━►┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃    Please send me a public key        ┃
//		        ┃                                        ┃◄--------------------------------------┃
//		        ┃    Request Biometrics / Device Pin     ┃                                       ┃
//		        ┃◄───────────────────────────────────────┃                                       ┃
//		        ┃                                        ┃                                       ┃
//		        ┃             User Verified              ┃                                       ┃
//		        ┃---------------------------------------►┃                                       ┃
//		        ┃                                        ┃  Cryptographic key pair               ┃
//		        ┃                                        ┃  generated                            ┃
//		        ┃                                        ┃─────────┐                             ┃
//		        ┃                                        ┃         │                             ┃
//		        ┃                                        ┃◄────────┘                             ┃
//		        ┃                                        ┃            Send public key            ┃
//		        ┃                                        ┃──────────────────────────────────────►┃
//		        ┃                                        ┃           Device registered           ┃
//		        ┃                                        ┃◄--------------------------------------┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃          ┌─────────────────┐          ┃
//	    ────────┃────────────────────────────────────────┃──────────│                 │──────────┃──────────────
//		────────┃────────────────────────────────────────┃──────────│ Authentication  │──────────┃──────────────
//		────────┃────────────────────────────────────────┃──────────│                 │──────────┃──────────────
//		        ┃                                        ┃          └─────────────────┘          ┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃          I want to login              ┃
//		        ┃                                        ┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━►┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃    Here's the challenge to prove      ┃
//		        ┃                                        ┃    your identity, sign this challenge ┃
//		        ┃                                        ┃◄--------------------------------------┃
//		        ┃    Request Biometrics / Device Pin     ┃                                       ┃
//		        ┃◄───────────────────────────────────────┃                                       ┃
//		        ┃             User Verified              ┃                                       ┃
//		        ┃---------------------------------------►┃                                       ┃
//		        ┃                                        ┃  Challenge signed with                ┃
//		        ┃                                        ┃  Private Key                          ┃
//		        ┃                                        ┃─────────┐                             ┃
//		        ┃                                        ┃         ┃                             ┃
//		        ┃                                        ┃◄────────┘                             ┃
//		        ┃                                        ┃        Send Signed challenge          ┃
//		        ┃                                        ┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━►┃
//		        ┃                                        ┃                                       ┃ Verify signature
//		        ┃                                        ┃                                       ┃ using Public Key
//		        ┃                                        ┃                                       ┃─────────┐
//		        ┃                                        ┃                                       ┃         │
//		        ┃                                        ┃                                       ┃◄────────┘
//		        ┃                                        ┃                 Success               ┃
//		        ┃                                        ┃◄--------------------------------------┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃                                       ┃
//		        ┃                                        ┃                                       ┃
//
// New returns a new Webauthn Service, which has simple wrappers for Signing up and registering a user
func New(cfg *config.WebAuthnConfig, store postgres.WebAuthN) WebAuthnService {
	core, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
		RPIcon:        cfg.RPIcon,
	})
	if err != nil {
		log.Fatalf("webauthn config is missing: %s", err)
	}

	return &webAuthnService{
		cfg:      cfg,
		store:    store,
		txnStore: make(map[string]*webAuthNMeta),
		core:     core,
	}
}

// BeginRegistration takes a WebAuthnUser and performs the "Server" logic on it. The actual work is done by the
// underlying webauthn library "github.com/go-webauthn/webauthn" but normal sanity checks are performed here like
// Only perform the "BeginRegistration" flow is the user doesn't already exist
func (wa *webAuthnService) BeginRegistration(
	ctx context.Context,
	user *WebAuthnUser,
) (*protocol.CredentialCreation, error) {
	creds, err := wa.store.GetWebAuthNCredentials(ctx, user.Id)
	if err != nil && errors.Unwrap(err) != pgx.ErrNoRows {
		return nil, err
	}

	// User might already have few credentials. They shouldn't be considered when creating a new credential for them.
	// A user can have multiple credentials
	excludeList := user.GetExistingPublicKeyCredentials()

	authSelect := &protocol.AuthenticatorSelection{
		AuthenticatorAttachment: protocol.Platform,
		RequireResidentKey:      protocol.ResidentKeyRequired(),
		UserVerification:        protocol.VerificationRequired,
	}

	conveyancePref := protocol.ConveyancePreference(protocol.PreferNoAttestation)

	user.AddWebAuthNCredentials(creds)
	credentialCreation, sessionData, err := wa.core.BeginRegistration(
		user,
		webauthn.WithExclusions(excludeList),
		webauthn.WithAuthenticatorSelection(*authSelect),
		webauthn.WithConveyancePreference(conveyancePref),
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_WEB_AUTHN_BEGIN_REGISTRATION: %w", err)
	}
	// store session data in DB
	if err = wa.store.AddWebAuthSessionData(ctx, user.Id, sessionData, "registration"); err != nil {
		return nil, err
	}

	return credentialCreation, err
}

func (wa *webAuthnService) RollbackRegisteration(ctx context.Context, username string) error {
	meta, ok := wa.txnStore[username]
	if !ok {
		return fmt.Errorf("ERR_ROLLBACK_REGISTRATION: txn does not exist")
	}

	err := meta.txn.Rollback(ctx)
	if err != nil {
		return err
	}

	return nil
}

type FinishRegistrationOpts struct {
	RequestBody io.Reader
	User        *WebAuthnUser
}

// FinishRegistration works like sort of a commit txn in database but in Webautnn context.
// A user must perform a BeginRegistration step before proceeding with this.
// Also, user is responsible for handling the failed and successful states for this, i.e, This method does not commit
// rollback your changes into the database. It only takes care of WebAuthn stuff
func (wa *webAuthnService) FinishRegistration(ctx context.Context, opts *FinishRegistrationOpts) error {
	sessionData, err := wa.store.GetWebAuthNSessionData(ctx, opts.User.Id, "registration")
	if err != nil {
		return err
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(opts.RequestBody)
	if err != nil {
		return err
	}

	credentials, err := wa.core.CreateCredential(opts.User, *sessionData, parsedResponse)
	if err != nil {
		return err
	}

	// append the credential to the User.credentials field
	opts.User.AddWebAuthNCredential(credentials)
	if err = wa.store.AddWebAuthNCredentials(ctx, opts.User.Id, credentials); err != nil {
		return err
	}

	return nil
}

type BeginLoginOptions struct {
	RequestBody io.Reader
	User        *WebAuthnUser
}

func (wa *webAuthnService) BeginLogin(
	ctx context.Context,
	opts *BeginLoginOptions,
) (*protocol.CredentialAssertion, error) {
	creds, err := wa.store.GetWebAuthNCredentials(ctx, opts.User.Id)
	if err != nil {
		return nil, err
	}

	// these credentials are added here because WebAuthn will try to access then via
	// user.WebAuthnCredentials method
	opts.User.AddWebAuthNCredential(creds)

	credentialAssertionOpts, sessionData, err := wa.core.BeginLogin(
		opts.User,
		webauthn.WithAllowedCredentials(opts.User.GetExistingPublicKeyCredentials()),
	)
	if err != nil {
		return nil, err
	}

	err = wa.store.AddWebAuthSessionData(ctx, opts.User.Id, sessionData, "authentication")
	if err != nil {
		return nil, err
	}

	return credentialAssertionOpts, nil
}

type FinishLoginOpts struct {
	RequestBody io.Reader
	User        *WebAuthnUser
}

// FinishLogin checks if begin login was performed successfully, parsed the request from the io.Reader,
// and then validates that request. If all is good, then we return nil, anything else, causes it to return an error
func (wa *webAuthnService) FinishLogin(ctx context.Context, opts *FinishLoginOpts) error {
	sessionData, err := wa.store.GetWebAuthNSessionData(ctx, opts.User.Id, "authentication")
	if err != nil {
		return err
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(opts.RequestBody)
	if err != nil {
		return err
	}

	creds, err := wa.store.GetWebAuthNCredentials(ctx, opts.User.Id)
	if err != nil {
		return err
	}

	opts.User.AddWebAuthNCredential(creds)

	//Validate login gives back credential
	_, err = wa.core.ValidateLogin(opts.User, *sessionData, parsedResponse)
	if err != nil {
		return err
	}

	return nil
}

func (wa *webAuthnService) doWebAuthnRegisteration(
	ctx context.Context,
	user *types.User,
) (*protocol.CredentialCreation, error) {
	creds, err := wa.store.GetWebAuthNCredentials(ctx, user.Id)
	if err != nil && errors.Unwrap(err) != pgx.ErrNoRows {
		return nil, err
	}

	// User might already have few credentials. They shouldn't be considered when creating a new credential for them.
	// A user can have multiple credentials
	excludeList := user.GetExistingPublicKeyCredentials()

	authSelect := &protocol.AuthenticatorSelection{
		AuthenticatorAttachment: protocol.Platform,
		RequireResidentKey:      protocol.ResidentKeyRequired(),
		UserVerification:        protocol.VerificationRequired,
	}

	conveyancePref := protocol.ConveyancePreference(protocol.PreferNoAttestation)

	user.AddWebAuthNCredentials(creds)
	credentialCreation, sessionData, err := wa.core.BeginRegistration(
		user,
		webauthn.WithExclusions(excludeList),
		webauthn.WithAuthenticatorSelection(*authSelect),
		webauthn.WithConveyancePreference(conveyancePref),
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_WEB_AUTHN_BEGIN_REGISTRATION: %w", err)
	}
	// store session data in DB
	if err = wa.store.AddWebAuthSessionData(ctx, user.Id, sessionData, "registration"); err != nil {
		return nil, err
	}

	return credentialCreation, err
}
