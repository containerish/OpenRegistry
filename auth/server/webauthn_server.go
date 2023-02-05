package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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
		webAuthN webauthn.WebAuthnService
		txnStore map[string]*webAuthNMeta
	}

	webAuthNMeta struct {
		expiresAt time.Time
		txn       pgx.Tx
	}

	WebauthnServer interface {
		BeginRegistration(ctx echo.Context) error
		RollbackRegisteration(ctx echo.Context) error
		FinishRegistration(ctx echo.Context) error
		BeginLogin(ctx echo.Context) error
		FinishLogin(ctx echo.Context) error
	}
)

func NewWebauthnServer(cfg *config.OpenRegistryConfig, store postgres.PersistentStore, logger telemetry.Logger) WebauthnServer {
	webauthnService := webauthn.New(&cfg.WebAuthnConfig, store)

	server := &webauthn_server{
		store:    store,
		logger:   logger,
		cfg:      cfg,
		webAuthN: webauthnService,
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
	var user types.User

	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid JSON object",
		})
		wa.logger.Log(ctx, err)
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
		wa.logger.Log(ctx, err)
		return echoErr
	}

	key := user.Email
	if user.Username != "" {
		key = user.Username
	}

	txn, err := wa.store.NewTxn(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "database error, failed to add user",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}

	wa.txnStore[user.Username] = &webAuthNMeta{
		txn:       txn,
		expiresAt: time.Now().Add(time.Minute),
	}

	existingUser, err := wa.store.GetUser(ctx.Request().Context(), key, true, nil)
	if err != nil {
		if errors.Unwrap(err) == pgx.ErrNoRows {
			//user does not exist, create new user
			user.Id = uuid.NewString()
			if err = wa.store.AddUser(ctx.Request().Context(), &user, txn); err != nil {
				echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
					"error":   err.Error(),
					"message": "database error, failed to add user",
				})
				wa.logger.Log(ctx, err)
				return echoErr
			}

			// set it here so that we can continue to use existingUser object
			existingUser = &user

		} else {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "database error, failed to get user",
			})
			wa.logger.Log(ctx, err)
			return echoErr
		}
	}

	webauthnUser := &webauthn.WebAuthnUser{User: existingUser}
	credentialOpts, err := wa.webAuthN.BeginRegistration(ctx.Request().Context(), webauthnUser)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "failed to add web authn session data for existing user",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "registration successful",
		"options": credentialOpts,
	})

	wa.logger.Log(ctx, echoErr)
	return echoErr
}

func (wa *webauthn_server) RollbackRegisteration(ctx echo.Context) error {
	username := ctx.QueryParam("username")
	meta, ok := wa.txnStore[username]
	if !ok {
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"message": "user txn does not exist",
		})

		wa.logger.Log(ctx, echoErr)
		return echoErr
	}

	err := meta.txn.Rollback(ctx.Request().Context())
	if err != nil {
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"error":   err.Error(),
			"message": "user txn does not exist",
		})

		wa.logger.Log(ctx, echoErr)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "txn rolled back successfully",
	})

	wa.logger.Log(ctx, echoErr)
	return nil
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

		wa.logger.Log(ctx, nil)
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

	if err = wa.webAuthN.FinishRegistration(ctx.Request().Context(), opts); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating webauthn credentials",
		})
		wa.logger.Log(ctx, err)
		return echoErr

	}
	defer ctx.Request().Body.Close()

	if err = meta.txn.Commit(ctx.Request().Context()); err != nil {
		_ = meta.txn.Rollback(ctx.Request().Context())
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error storing the credential info",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "registration successful",
	})

	wa.logger.Log(ctx, echoErr)
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
		wa.logger.Log(ctx, err)
		return echoErr
	}

	opts := &webauthn.BeginLoginOptions{
		RequestBody: ctx.Request().Body,
		User: &webauthn.WebAuthnUser{
			User: user,
		},
	}

	credentialAssertion, err := wa.webAuthN.BeginLogin(ctx.Request().Context(), opts)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error performing Webauthn login",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}
	defer ctx.Request().Body.Close()

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"options": credentialAssertion,
	})

	wa.logger.Log(ctx, echoErr)
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
		wa.logger.Log(ctx, err)
		return echoErr
	}

	opts := &webauthn.FinishLoginOpts{
		RequestBody: ctx.Request().Body,
		User: &webauthn.WebAuthnUser{
			User: user,
		},
	}

	if err = wa.webAuthN.FinishLogin(ctx.Request().Context(), opts); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "parsing error: could not parse credential request body in finish login",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}
	defer ctx.Request().Body.Close()

	access, err := wa.newWebLoginToken(user.Id, user.Username, "access")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating web login token",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}

	refresh, err := wa.newWebLoginToken(user.Id, user.Username, "refresh")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating refresh token",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}
	id := uuid.NewString()
	sessionId := fmt.Sprintf("%s:%s", id, user.Id)

	if err = wa.store.AddSession(ctx.Request().Context(), id, refresh, user.Username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error creating session",
		})
		wa.logger.Log(ctx, err)
		return echoErr
	}

	sessionCookie := wa.createCookie("session_id", sessionId, false, time.Now().Add(time.Hour*750))
	accessCookie := wa.createCookie("access", access, true, time.Now().Add(time.Hour*750))
	refreshCookie := wa.createCookie("refresh", refresh, true, time.Now().Add(time.Hour*750))
	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "Login Success",
	})

	wa.logger.Log(ctx, echoErr)
	return echoErr
}
