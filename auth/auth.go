package auth

import (
	"log"
	"time"

	"github.com/duo-labs/webauthn/webauthn"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	gh "github.com/google/go-github/v42/github"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// Authentication interface defines the behaviour for container registry and general authentication for the backend
type Authentication interface {
	SignUp(ctx echo.Context) error
	SignIn(ctx echo.Context) error
	BasicAuth() echo.MiddlewareFunc
	Token(ctx echo.Context) error
	JWT() echo.MiddlewareFunc
	JWTRest() echo.MiddlewareFunc
	ACL() echo.MiddlewareFunc
	LoginWithGithub(ctx echo.Context) error
	GithubLoginCallbackHandler(ctx echo.Context) error
	ExpireSessions(ctx echo.Context) error
	SignOut(ctx echo.Context) error
	ReadUserWithSession(ctx echo.Context) error
	RenewAccessToken(ctx echo.Context) error
	VerifyEmail(ctx echo.Context) error
	ResetPassword(ctx echo.Context) error
	ResetForgottenPassword(ctx echo.Context) error
	ForgotPassword(ctx echo.Context) error
	Invites(ctx echo.Context) error
	BeginRegistration(ctx echo.Context) error
	FinishRegistration(ctx echo.Context) error
	BeginLogin(ctx echo.Context) error
	FinishLogin(ctx echo.Context) error
}

// New is the constructor function returns an Authentication implementation
func New(
	c *config.OpenRegistryConfig,
	pgStore postgres.PersistentStore,
	logger telemetry.Logger,
) Authentication {

	githubOAuth := &oauth2.Config{
		ClientID:     c.OAuth.Github.ClientID,
		ClientSecret: c.OAuth.Github.ClientSecret,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"user:email"},
	}

	ghClient := gh.NewClient(nil)
	emailClient := email.New(c.Email, c.WebAppEndpoint)
	webAuthN, err := webauthn.New(&webauthn.Config{
		RPDisplayName: c.WebAuthnConfig.RPDisplayName,
		RPID:          c.WebAuthnConfig.RPID,
		RPOrigin:      c.WebAuthnConfig.RPOrigin,
		RPIcon:        c.WebAuthnConfig.RPIcon,
	})
	if err != nil {
		log.Fatalf("webauthn config is missing")
	}

	a := &auth{
		c:               c,
		pgStore:         pgStore,
		logger:          logger,
		github:          githubOAuth,
		ghClient:        ghClient,
		oauthStateStore: make(map[string]time.Time),
		webAuthN:        webAuthN,
		emailClient:     emailClient,
	}

	go a.StateTokenCleanup()

	return a
}

type (
	auth struct {
		pgStore         postgres.PersistentStore
		logger          telemetry.Logger
		github          *oauth2.Config
		ghClient        *gh.Client
		oauthStateStore map[string]time.Time
		c               *config.OpenRegistryConfig
		webAuthN        *webauthn.WebAuthn
		emailClient     email.MailService
	}
)

// @TODO (jay-dee7) maybe a better way to do it?
func (a *auth) StateTokenCleanup() {
	// tick every 10 minutes, delete ant oauth state tokens which are older than 10 mins
	// duration = 10mins, because github short lived code is valid for 10 mins
	for range time.Tick(time.Second * 10) {
		for key, t := range a.oauthStateStore {
			if time.Now().Unix() > t.Unix() {
				delete(a.oauthStateStore, key)
			}
		}
	}
}
