package github

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/fatih/color"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
)

type ghAppService struct {
	store                vcs.VCSStore
	logger               telemetry.Logger
	config               *config.GithubIntegration
	ghClient             *github.Client
	ghAppTransport       *ghinstallation.AppsTransport
	automationBranchName string
	workflowFilePath     string
	registryEndpoint     string
	webInterfaceURLs     []string
	env                  config.Environment
}

func NewGithubApp(
	cfg *config.GithubIntegration,
	store vcs.VCSStore,
	logger telemetry.Logger,
	webInterfaceURLs []string,
	env config.Environment,
	registryEndpoint string,
) vcs.VCS {
	ghAppTransport, ghClient, err := newGHClient(cfg.AppID, cfg.PrivateKeyPem)
	if err != nil {
		log.Fatalln(color.RedString("ERR_CREATE_NEW_GH_CLIENT: %w", err))
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
		registryEndpoint:     registryEndpoint,
	}
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
func (gh *ghAppService) RegisterRoutes(router *echo.Group) {
	router.Use(
		gh.getUsernameMiddleware(),
		gh.getGitubInstallationID(vcs.HandleAppFinishEndpoint, vcs.HandleSetupCallbackEndpoint),
	)

	router.Add(http.MethodGet, vcs.ListAuthorisedRepositoriesEndpoint, gh.ListAuthorisedRepositories)
	router.Add(http.MethodGet, vcs.HandleSetupCallbackEndpoint, gh.HandleSetupCallback)
	router.Add(http.MethodPost, vcs.HandleWebhookEventsEndpoint, gh.HandleWebhookEvents)
	router.Add(http.MethodPost, vcs.HandleAppFinishEndpoint, gh.HandleAppFinish)
	router.Add(http.MethodPost, vcs.CreateInitialPREndpoint, gh.CreateInitialPR)
}

func (gh *ghAppService) getUsernameMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			// skip if it's a webhook call
			// if c.Path() == "/github"+vcs.HandleWebhookEventsEndpoint || c.Path() == "/github/app/callback" {
			if ctx.Path() == "/github"+vcs.HandleWebhookEventsEndpoint {
				return next(ctx)
			}

			sessionCookie, err := ctx.Cookie("session_id")
			if err != nil {
				echoErr := ctx.JSON(http.StatusNotAcceptable, echo.Map{
					"error":     err.Error(),
					"cookie_id": "session_id",
				})
				gh.logger.Log(ctx, err).Send()
				return echoErr
			}

			// session is <session_uuid>:<userid>, and ":" is url encoded
			sessionID, err := url.QueryUnescape(sessionCookie.Value)
			if err != nil {
				echoErr := ctx.JSON(http.StatusNotAcceptable, echo.Map{
					"error":     err.Error(),
					"cookie_id": "session_id",
				})
				gh.logger.Log(ctx, err).Send()
				return echoErr
			}
			userID := strings.Split(sessionID, ":")[1]
			parsedID, err := uuid.Parse(userID)
			if err != nil {
				echoErr := ctx.JSON(http.StatusForbidden, echo.Map{
					"error": fmt.Errorf("ERR_PARSE_USER_ID: %w", err),
				})
				gh.logger.Log(ctx, err).Send()
				return echoErr
			}

			user, err := gh.store.GetUserByID(ctx.Request().Context(), parsedID)
			if err != nil {
				echoErr := ctx.JSON(http.StatusForbidden, echo.Map{
					"error": fmt.Errorf("ERR_GET_AUTHZ_USER: %w", err),
				})
				gh.logger.Log(ctx, err).Send()
				return echoErr
			}

			ctx.Set(string(types.UserContextKey), user)
			return next(ctx)
		}
	}
}

func (gh *ghAppService) getGitubInstallationID(skipRoutes ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() == "/github"+vcs.HandleWebhookEventsEndpoint {
				return next(c)
			}
			user, ok := c.Get(string(types.UserContextKey)).(*types.User)
			if !ok {
				err := fmt.Errorf("GH_MDW_ERR: username is not present in context")
				echoErr := c.JSON(http.StatusNotAcceptable, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
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

			if user.Identities == nil || user.Identities.GetGitHubIdentity() == nil {
				err := fmt.Errorf("GH_MDW_ERR: GitHub identity not found")
				echoErr := c.JSON(http.StatusBadRequest, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
				return echoErr
			}

			c.Set(string(GithubInstallationIDContextKey), user.Identities.GetGitHubIdentity().InstallationID)
			return next(c)
		}
	}
}

func newGHClient(appID int64, privKeyPem string) (*ghinstallation.AppsTransport, *github.Client, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, privKeyPem)
	if err != nil {
		return nil, nil, fmt.Errorf("ERR_CREATE_NEW_TRANSPORT: %w - file: %s", err, privKeyPem)
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
	GithubInstallationIDContextKey ContextKey = "GITHUB_INSTALLATION_ID"

	WorkflowFilePath                 = ".github/workflows/openregistry.yml"
	OpenRegistryAutomationBranchName = "openregistry-build-automation"
	MaxGitHubRedirects               = 3
)
