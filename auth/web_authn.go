package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

func (a *auth) BeginRegistration(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid JSON object",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	_ = ctx.Request().Body.Close()

	err := user.Validate(false)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid data provided for user login",
			"code":    "INVALID_CREDENTIALS",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	key := user.Email
	if user.Username != "" {
		key = user.Username
	}

	txn, err := a.pgStore.NewTxn(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error, failed to add user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	a.txnStore[user.Username] = &webAuthNMeta{
		txn:       txn,
		expiresAt: time.Now().Add(time.Minute),
	}

	userFromDb, err := a.pgStore.GetUser(ctx.Request().Context(), key, true, nil)
	if err != nil {
		if errors.Unwrap(err) == pgx.ErrNoRows {
			//user does not exist, create new user
			user.Id = uuid.NewString()
			if err = a.pgStore.AddUser(ctx.Request().Context(), &user, txn); err != nil {
				echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
					"error":   err.Error(),
					"message": "database error, failed to add user",
				})
				a.logger.Log(ctx, err)
				return echoErr
			}

			// set it here so that we can continue to use userFromDb object
			userFromDb = &user

		} else {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "database error, failed to get user",
			})
			a.logger.Log(ctx, err)
			return echoErr
		}
	}

	credentialOpts, err := a.doWebAuthnRegisteration(ctx.Request().Context(), userFromDb)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "failed to add web authn session data for existing user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "registration successful",
		"options": credentialOpts,
	})

	a.logger.Log(ctx, echoErr)
	return echoErr
}

func (a *auth) RollbackRegisteration(ctx echo.Context) error {
	username := ctx.QueryParam("username")
	meta, ok := a.txnStore[username]
	if !ok {
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"message": "user txn does not exist",
		})

		a.logger.Log(ctx, echoErr)
		return echoErr
	}

	err := meta.txn.Rollback(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"error":   err.Error(),
			"message": "user txn does not exist",
		})

		a.logger.Log(ctx, echoErr)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "txn rolled back successfully",
	})

	a.logger.Log(ctx, echoErr)
	return nil
}

func (a *auth) FinishRegistration(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	meta, ok := a.txnStore[username]
	if !ok {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   "missing begin registration step",
			"message": "no user found with this username",
		})

		a.logger.Log(ctx, nil)
		return echoErr
	}

	userFromDB, err := a.pgStore.GetUser(ctx.Request().Context(), username, false, meta.txn)
	if err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "no user found with this username",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionData, err := a.pgStore.GetWebAuthNSessionData(ctx.Request().Context(), userFromDB.Id, "registration")
	if err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())

		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error, session data not found",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(ctx.Request().Body)
	if err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing credential creation response body",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	defer ctx.Request().Body.Close()

	credentials, err := a.webAuthN.CreateCredential(userFromDB, *sessionData, parsedResponse)
	if err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating webauthn credentials",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	// append the credential to the User.credentials field
	userFromDB.AddWebAuthNCredential(credentials)
	if err = a.pgStore.AddWebAuthNCredentials(ctx.Request().Context(), userFromDB.Id, credentials); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "database error storing webauthn credentials",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if err = meta.txn.Commit(ctx.Request().Context()); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error storing the credential info",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "registration successful",
	})

	a.logger.Log(ctx, echoErr)
	return echoErr
}

func (a *auth) BeginLogin(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	userFromDB, err := a.pgStore.GetUser(ctx.Request().Context(), username, false, nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "database error: user not found",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	creds, err := a.pgStore.GetWebAuthNCredentials(ctx.Request().Context(), userFromDB.Id)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting credentials for user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	// these credentials are added here because WebAuthn will try to access then via
	// user.WebAuthnCredentials method
	userFromDB.AddWebAuthNCredential(creds)

	credentialAssertionOpts, sessionData, err := a.webAuthN.BeginLogin(
		userFromDB,
		webauthn.WithAllowedCredentials(userFromDB.GetExistingPublicKeyCredentials()),
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error begin login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	err = a.pgStore.AddWebAuthSessionData(ctx.Request().Context(), userFromDB.Id, sessionData, "authentication")
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error: storing session data while web authn begin login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"options": &credentialAssertionOpts,
	})
	a.logger.Log(ctx, echoErr)
	return echoErr
}

func (a *auth) FinishLogin(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	userFromDb, err := a.pgStore.GetUser(ctx.Request().Context(), username, false, nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error: user not found",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionData, err := a.pgStore.GetWebAuthNSessionData(ctx.Request().Context(), userFromDb.Id, "authentication")
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error: session data for user not found in finish login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(ctx.Request().Body)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "parsing error: could not parse credential request body in finish login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	defer ctx.Request().Body.Close()

	creds, err := a.pgStore.GetWebAuthNCredentials(ctx.Request().Context(), userFromDb.Id)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error getting credentials for user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	userFromDb.AddWebAuthNCredential(creds)

	//Validate login gives back credential
	_, err = a.webAuthN.ValidateLogin(userFromDb, *sessionData, parsedResponse)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "could not validate user login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	access, err := a.newWebLoginToken(userFromDb.Id, userFromDb.Username, "access")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating web login token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	refresh, err := a.newWebLoginToken(userFromDb.Id, userFromDb.Username, "refresh")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating refresh token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	id := uuid.NewString()
	sessionId := fmt.Sprintf("%s:%s", id, userFromDb.Id)

	if err = a.pgStore.AddSession(ctx.Request().Context(), id, refresh, userFromDb.Username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionCookie := a.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", access, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))
	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "Login Success",
	})

	a.logger.Log(ctx, echoErr)
	return echoErr
}

func (a *auth) doWebAuthnRegisteration(ctx context.Context, user *types.User) (*protocol.CredentialCreation, error) {
	creds, err := a.pgStore.GetWebAuthNCredentials(ctx, user.Id)
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
	credentialCreation, sessionData, err := a.webAuthN.BeginRegistration(
		user,
		webauthn.WithExclusions(excludeList),
		webauthn.WithAuthenticatorSelection(*authSelect),
		webauthn.WithConveyancePreference(conveyancePref),
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_WEB_AUTHN_BEGIN_REGISTRATION: %w", err)
	}
	// store session data in DB
	if err = a.pgStore.AddWebAuthSessionData(ctx, user.Id, sessionData, "registration"); err != nil {
		return nil, err
	}

	return credentialCreation, err
}
