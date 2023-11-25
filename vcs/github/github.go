package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ghAppService struct {
	store                vcs.VCSStore
	logger               telemetry.Logger
	config               *config.Integration
	ghClient             *github.Client
	ghAppTransport       *ghinstallation.AppsTransport
	automationBranchName string
	workflowFilePath     string
	webInterfaceURLs     []string
	env                  config.Environment
}

func NewGithubApp(
	cfg *config.Integration,
	store vcs.VCSStore,
	logger telemetry.Logger,
	webInterfaceURLs []string,
	env config.Environment,
) (vcs.VCS, error) {
	ghAppTransport, ghClient, err := newGHClient(cfg.AppID, cfg.PrivateKeyPem)
	if err != nil {
		return nil, fmt.Errorf("ERR_CREATE_NEW_GH_CLIENT: %w", err)
	}

	return &ghAppService{
		config:               cfg,
		store:                store,
		workflowFilePath:     WorkflowFilePath,
		ghClient:             ghClient,
		ghAppTransport:       ghAppTransport,
		logger:               logger,
		automationBranchName: OpenRegistryAutomationBranchName,
		webInterfaceURLs:     webInterfaceURLs,
		env:                  env,
	}, nil
}

// return any methods which can be called as APIs from an http client
func (gh *ghAppService) ListHandlers() []echo.HandlerFunc {
	return []echo.HandlerFunc{
		gh.HandleSetupCallback,
		gh.HandleWebhookEvents,
		gh.ListAuthorisedRepositories,
		gh.HandleAppFinish,
		gh.CreateInitialPR,
	}
}

// RegisterRoutes takes in a echo.Group (aka sub router) which is prefix with VCS name
// eg: for GitHub, the sub router would be prefixed with "/github"
func (gh *ghAppService) RegisterRoutes(r *echo.Group) {
	r.Use(
		gh.getUsernameMiddleware(),
		gh.getGitubInstallationID(vcs.HandleAppFinishEndpoint, vcs.HandleSetupCallbackEndpoint),
	)

	r.Add(http.MethodGet, vcs.ListAuthorisedRepositoriesEndpoint, gh.ListAuthorisedRepositories)
	r.Add(http.MethodGet, vcs.HandleSetupCallbackEndpoint, gh.HandleSetupCallback)
	r.Add(http.MethodPost, vcs.HandleWebhookEventsEndpoint, gh.HandleWebhookEvents)
	r.Add(http.MethodPost, vcs.HandleAppFinishEndpoint, gh.HandleAppFinish)
	r.Add(http.MethodPost, vcs.CreateInitialPREndpoint, gh.CreateInitialPR)
}

func (gh *ghAppService) getUsernameMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/github"+vcs.HandleWebhookEventsEndpoint {
				return next(c)
			}
			sessionCookie, err := c.Cookie("session_id")
			if err != nil {
				echoErr := c.JSON(http.StatusNotAcceptable, echo.Map{
					"error":     err.Error(),
					"cookie_id": "session_id",
				})
				gh.logger.Log(c, err).Send()
				return echoErr
			}

			// session is <session_uuid>:<userid>, and ":" is url encoded
			sessionID, err := url.QueryUnescape(sessionCookie.Value)
			if err != nil {
				echoErr := c.JSON(http.StatusNotAcceptable, echo.Map{
					"error":     err.Error(),
					"cookie_id": "session_id",
				})
				gh.logger.Log(c, err).Send()
				return echoErr
			}
			userID := strings.Split(sessionID, ":")[1]
			parsedID, err := uuid.Parse(userID)
			if err != nil {
				echoErr := c.JSON(http.StatusForbidden, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
				return echoErr
			}
			user, err := gh.store.GetUserByID(c.Request().Context(), parsedID)
			if err != nil {
				echoErr := c.JSON(http.StatusNotAcceptable, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
				return echoErr
			}

			c.Set(string(UsernameContextKey), user.Username)
			c.Set(string(UserContextKey), user)
			return next(c)
		}
	}
}

func (gh *ghAppService) getGitubInstallationID(skipRoutes ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/github"+vcs.HandleWebhookEventsEndpoint {
				return next(c)
			}
			user, ok := c.Get(string(UserContextKey)).(*types.User)
			if !ok {
				echoErr := c.JSON(http.StatusNotAcceptable, echo.Map{
					"error": "GH_MDW_ERR: username is not present in context",
				})
				gh.logger.Log(c, echoErr).Send()
				return echoErr
			}

			skip := false
			for _, r := range skipRoutes {
				if c.Request().URL.Path == "/github"+r {
					skip = true
				}
			}

			if skip {
				return next(c)
			}

			c.Set(string(GithubInstallationIDContextKey), user.Identities.GetGitHubIdentity().InstallationID)
			return next(c)
		}
	}
}

func newGHClient(appID int64, privKeyPem string) (*ghinstallation.AppsTransport, *github.Client, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, privKeyPem)
	if err != nil {
		return nil, nil, fmt.Errorf("ERR_CREATE_NEW_TRANSPORT: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport, Timeout: time.Second * 30})
	return transport, client, nil
}

func (gh *ghAppService) refreshGHClient(id int64) *github.Client {
	transport := ghinstallation.NewFromAppsTransport(gh.ghAppTransport, id)
	return github.NewClient(&http.Client{Transport: transport})
}

type AuthorizedRepository struct {
	Repository *github.Repository `json:"repository"`
	Branches   []*github.Branch   `json:"branches"`
}

type ContextKey string

const (
	UsernameContextKey               ContextKey = "USERNAME"
	UserContextKey                   ContextKey = "USER"
	GithubInstallationIDContextKey   ContextKey = "GITHUB_INSTALLATION_ID"
	WorkflowFilePath                            = ".github/workflows/openregistry.yml"
	OpenRegistryAutomationBranchName            = "openregistry-build-automation"
	MaxGitHubRedirects                          = 3
)
