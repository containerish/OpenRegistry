package auth

import (
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/store/v1/permissions"
	"github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	gh "github.com/google/go-github/v56/github"
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
	RepositoryPermissionsMiddleware() echo.MiddlewareFunc
}

// New is the constructor function returns an Authentication implementation
func New(
	c *config.OpenRegistryConfig,
	userStore users.UserStore,
	sessionStore users.SessionStore,
	emailStore users.EmailStore,
	registryStore registry.RegistryStore,
	permissionsStore permissions.PermissionsStore,
	logger telemetry.Logger,
) Authentication {
	githubOAuth := &oauth2.Config{
		ClientID:     c.OAuth.Github.ClientID,
		ClientSecret: c.OAuth.Github.ClientSecret,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"user:email"},
	}

	ghClient := gh.NewClient(nil)
	emailClient := email.New(&c.Email)

	a := &auth{
		c:                c,
		logger:           logger,
		github:           githubOAuth,
		ghClient:         ghClient,
		oauthStateStore:  make(map[string]time.Time),
		mu:               &sync.RWMutex{},
		emailClient:      emailClient,
		userStore:        userStore,
		sessionStore:     sessionStore,
		emailStore:       emailStore,
		registryStore:    registryStore,
		permissionsStore: permissionsStore,
	}

	go a.stateTokenCleanup()

	return a
}

type (
	auth struct {
		logger           telemetry.Logger
		github           *oauth2.Config
		ghClient         *gh.Client
		oauthStateStore  map[string]time.Time
		emailClient      email.MailService
		mu               *sync.RWMutex
		c                *config.OpenRegistryConfig
		userStore        users.UserStore
		sessionStore     users.SessionStore
		emailStore       users.EmailStore
		registryStore    registry.RegistryStore
		permissionsStore permissions.PermissionsStore
	}
)

// @TODO (jay-dee7) maybe a better way to do it?
func (a *auth) stateTokenCleanup() {
	// tick every 10 seconds, delete any oauth state tokens which are older than 10 mins
	// duration = 10mins, because github short lived code is valid for 10 mins
	for range time.Tick(time.Second * 10) {
		a.mu.Lock()
		for key, t := range a.oauthStateStore {
			if time.Now().Unix() > t.Unix() {
				delete(a.oauthStateStore, key)
			}
		}
		a.mu.Unlock()
	}
}
