package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/auth/webauthn"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

type (
	webauthn_server struct {
		store    postgres.PersistentStore
		logger   telemetry.Logger
		cfg      *config.OpenRegistryConfig
		webauthn webauthn.WebAuthnService
		txnStore map[string]*webAuthNMeta
	}

	webAuthNMeta struct {
		expiresAt time.Time
		txn       pgx.Tx
	}

	WebauthnServer interface {
		BeginRegistration(ctx echo.Context) error
		FinishRegistration(ctx echo.Context) error
		BeginLogin(ctx echo.Context) error
		FinishLogin(ctx echo.Context) error
		RollbackRegistration(ctx echo.Context) error
		RollbackSessionData(ctx echo.Context) error
	}
)

func NewWebauthnServer(
	cfg *config.OpenRegistryConfig,
	store postgres.PersistentStore,
	logger telemetry.Logger,
) WebauthnServer {
	webauthnService := webauthn.New(&cfg.WebAuthnConfig, store)

	server := &webauthn_server{
		store:    store,
		logger:   logger,
		cfg:      cfg,
		webauthn: webauthnService,
		txnStore: make(map[string]*webAuthNMeta),
	}

	go server.webAuthNTxnCleanup()
	return server
}

func (wa *webauthn_server) webAuthNTxnCleanup() {
	for range time.Tick(time.Second * 2) {
		for username, meta := range wa.txnStore {
			if meta.expiresAt.Unix() <= time.Now().Unix() {
				_ = meta.txn.Rollback(context.Background())
				delete(wa.txnStore, username)
			}
		}
	}
}

func (wa *webauthn_server) BeginRegistration(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	user := types.User{}

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid JSON object",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}
	_ = ctx.Request().Body.Close()
	user.Identities = make(types.Identities)

	err := user.Validate(false)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid data provided for user login",
			"code":    "INVALID_CREDENTIALS",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	wa.invalidateExistingRequests(ctx.Request().Context(), user.Username)
	txn, err := wa.store.NewTxn(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error, failed to add user",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	wa.txnStore[user.Username] = &webAuthNMeta{
		txn:       txn,
		expiresAt: time.Now().Add(time.Minute),
	}

	_, err = wa.store.GetUser(ctx.Request().Context(), user.Email, false, nil)
	if errors.Unwrap(err) == pgx.ErrNoRows {
		user.Id = uuid.NewString()
		user.IsActive = true
		user.WebauthnConnected = true
		user.Identities[types.IdentityProviderWebauthn] = &types.UserIdentity{
			ID:       user.Id,
			Username: user.Username,
			Email:    user.Email,
		}
		if err = wa.store.AddUser(ctx.Request().Context(), &user, txn); err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "failed to store user details",
			})
			wa.logger.Log(ctx, err).Send()
			return echoErr
		}

		webauthnUser := &webauthn.WebAuthnUser{User: &user}
		credentialOpts, wErr := wa.webauthn.BeginRegistration(ctx.Request().Context(), webauthnUser)
		if wErr != nil {
			// If we encounter an error here, we need to do the following:
			// 1. Rollback the session data (since this session data is irrelevant from this point onwards)
			// 2. Rollback the webauthn user store txn
			if werr := wa.webauthn.RemoveSessionData(ctx.Request().Context(), user.Id); werr != nil {
				echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
					"error":   werr.Error(),
					"message": "failed to rollback stale session data",
				})
				wa.logger.Log(ctx, wErr).Send()
				return echoErr
			}

			if rollbackErr := txn.Rollback(ctx.Request().Context()); rollbackErr != nil {
				echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
					"error":   rollbackErr.Error(),
					"message": "failed to rollback webauthn user txn",
				})
				wa.logger.Log(ctx, wErr).Send()
				return echoErr
			}

			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   wErr.Error(),
				"message": "failed to add webauthn session data for existing user",
			})
			wa.logger.Log(ctx, wErr).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"message": "registration successful",
			"options": credentialOpts,
		})

		wa.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	err = fmt.Errorf("username/email already exists")
	echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
		"error":   err.Error(),
		"message": "username/email already exists",
	})
	wa.logger.Log(ctx, err).Send()
	return echoErr
}

func (wa *webauthn_server) RollbackRegistration(ctx echo.Context) error {
	username := ctx.QueryParam("username")
	meta, ok := wa.txnStore[username]
	if !ok {
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"message": "user transaction does not exist",
		})

		wa.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	err := meta.txn.Rollback(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "failed to rollback transaction",
		})

		wa.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "transaction rolled back successfully",
	})

	wa.logger.Log(ctx, echoErr).Send()
	return nil
}

func (wa *webauthn_server) RollbackSessionData(ctx echo.Context) error {
	username := ctx.QueryParam("username")
	if username == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request, missing username",
		})
	}

	user, err := wa.store.GetUser(ctx.Request().Context(), username, false, nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "no user found",
		})

		wa.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	err = wa.webauthn.RemoveSessionData(ctx.Request().Context(), user.Id)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error rolling back session data for webauthn login",
		})

		wa.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	echoErr := ctx.NoContent(http.StatusNoContent)
	wa.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (wa *webauthn_server) FinishRegistration(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	meta, ok := wa.txnStore[username]
	if !ok {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   "missing begin registration step",
			"message": "no user found with this username",
		})

		wa.logger.Log(ctx, nil).Send()
		return echoErr
	}

	user, err := wa.store.GetUser(ctx.Request().Context(), username, false, meta.txn)
	if err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "no user found with this username",
		})
		return echoErr
	}

	opts := &webauthn.FinishRegistrationOpts{
		RequestBody: ctx.Request().Body,
		User: &webauthn.WebAuthnUser{
			User: user,
		},
	}

	if err = wa.webauthn.FinishRegistration(ctx.Request().Context(), opts); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating webauthn credentials",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr

	}
	defer ctx.Request().Body.Close()

	if err = meta.txn.Commit(ctx.Request().Context()); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error storing the credential info",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "registration successful",
	})

	wa.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (wa *webauthn_server) BeginLogin(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	user, err := wa.store.GetUser(ctx.Request().Context(), username, false, nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error: user not found",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	opts := &webauthn.BeginLoginOptions{
		RequestBody: ctx.Request().Body,
		User: &webauthn.WebAuthnUser{
			User: user,
		},
	}

	credentialAssertion, err := wa.webauthn.BeginLogin(ctx.Request().Context(), opts)
	if err != nil {
		if werr := wa.webauthn.RemoveSessionData(ctx.Request().Context(), user.Id); werr != nil {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   werr.Error(),
				"message": "error removing webauthn session data",
			})
			wa.logger.Log(ctx, werr).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error performing Webauthn login",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"options": credentialAssertion,
	})

	wa.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (wa *webauthn_server) FinishLogin(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	username := ctx.QueryParam("username")
	user, err := wa.store.GetUser(ctx.Request().Context(), username, false, nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error: user not found",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	opts := &webauthn.FinishLoginOpts{
		RequestBody: ctx.Request().Body,
		User: &webauthn.WebAuthnUser{
			User: user,
		},
	}

	if err = wa.webauthn.FinishLogin(ctx.Request().Context(), opts); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "parsing error: could not parse credential request body in finish login",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	accessTokenOpts := &auth.WebLoginJWTOptions{
		Id:        user.Id,
		Username:  username,
		TokenType: "access_token",
		Audience:  wa.cfg.Registry.FQDN,
		Privkey:   wa.cfg.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    wa.cfg.Registry.Auth.JWTSigningPubKey,
	}

	refreshTokenOpts := &auth.WebLoginJWTOptions{
		Id:        user.Id,
		Username:  username,
		TokenType: "refresh_token",
		Audience:  wa.cfg.Registry.FQDN,
		Privkey:   wa.cfg.Registry.Auth.JWTSigningPrivateKey,
		Pubkey:    wa.cfg.Registry.Auth.JWTSigningPubKey,
	}

	accessToken, err := auth.NewWebLoginToken(accessTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating web login token",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	refreshToken, err := auth.NewWebLoginToken(refreshTokenOpts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating refresh token",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}
	id := uuid.NewString()
	sessionId := fmt.Sprintf("%s:%s", id, user.Id)

	if err = wa.store.AddSession(ctx.Request().Context(), id, refreshToken, user.Username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		wa.logger.Log(ctx, err).Send()
		return echoErr
	}

	domain := ""
	url, err := url.Parse(wa.cfg.WebAuthnConfig.GetAllowedURLFromEchoContext(ctx, wa.cfg.Environment))
	if err != nil {
		domain = wa.cfg.WebAuthnConfig.RPOrigins[0]
	} else {
		domain = url.Hostname()
	}

	sessionIdCookie := auth.CreateCookie(&auth.CreateCookieOptions{
		ExpiresAt:   time.Now().Add(time.Hour * 750), //one month
		Name:        "session_id",
		Value:       sessionId,
		FQDN:        domain,
		Environment: wa.cfg.Environment,
		HTTPOnly:    false,
	})

	accessTokenCookie := auth.CreateCookie(&auth.CreateCookieOptions{
		ExpiresAt:   time.Now().Add(time.Hour * 750),
		Name:        "access_token",
		Value:       accessToken,
		FQDN:        domain,
		Environment: wa.cfg.Environment,
		HTTPOnly:    true,
	})

	refreshTokenCookie := auth.CreateCookie(&auth.CreateCookieOptions{
		ExpiresAt:   time.Now().Add(time.Hour * 750), //one month
		Name:        "refresh_token",
		Value:       refreshToken,
		FQDN:        domain,
		Environment: wa.cfg.Environment,
		HTTPOnly:    true,
	})

	ctx.SetCookie(accessTokenCookie)
	ctx.SetCookie(refreshTokenCookie)
	ctx.SetCookie(sessionIdCookie)

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "Login Success",
	})

	wa.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (wa *webauthn_server) invalidateExistingRequests(ctx context.Context, username string) {
	meta, ok := wa.txnStore[username]
	if ok {
		_ = meta.txn.Rollback(ctx)
	}
}
