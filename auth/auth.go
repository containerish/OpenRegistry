package auth

import (
	"fmt"

	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
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
	ACL() echo.MiddlewareFunc
	LoginWithGithub(ctx echo.Context) error
	GithubLoginCallbackHandler(ctx echo.Context) error
}

type auth struct {
	pgStore         postgres.PersistentStore
	store           cache.Store
	logger          telemetry.Logger
	github          *oauth2.Config
	ghClient        *gh.Client
	c               *config.OpenRegistryConfig
	oauthStateToken string
}

// New is the constructor function returns an Authentication implementation
func New(
	s cache.Store,
	c *config.OpenRegistryConfig,
	pgStore postgres.PersistentStore,
	logger telemetry.Logger,
) Authentication {

	githubOAuth := &oauth2.Config{
		ClientID:     c.OAuth.Github.ClientID,
		ClientSecret: c.OAuth.Github.ClientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  fmt.Sprintf("%s/auth/github/callback", c.Endpoint()),
		Scopes:       []string{"user:email"},
	}

	ghClient := gh.NewClient(nil)

	a := &auth{
		store:           s,
		c:               c,
		pgStore:         pgStore,
		logger:          logger,
		github:          githubOAuth,
		ghClient:        ghClient,
		oauthStateToken: "skljdfkljsdjfklj",
	}

	return a
}
