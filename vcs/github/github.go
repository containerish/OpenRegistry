package github

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/google/go-github/v46/github"
	"github.com/labstack/echo/v4"
)

type ghAppService struct {
	config               *config.Integation
	store                vcs.VCSStore
	ghClient             *github.Client
	ghAppTransport       *ghinstallation.AppsTransport
	logger               telemetry.Logger
	automationBranchName string
	workflowFilePath     string
}

func NewGithubApp(cfg *config.Integation, store vcs.VCSStore, logger telemetry.Logger) (vcs.VCS, error) {
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
	r.Use(gh.getUsernameMiddleware())

	r.Add(http.MethodGet, vcs.ListAuthorisedRepositoriesEndpoint, gh.ListAuthorisedRepositories)
	r.Add(http.MethodGet, vcs.HandleSetupCallbackEndpoint, gh.HandleSetupCallback)
	r.Add(http.MethodPost, vcs.HandleWebhookEventsEndpoint, gh.HandleWebhookEvents)
	r.Add(http.MethodPost, vcs.HandleAppFinishEndpoint, gh.HandleAppFinish)
	r.Add(http.MethodPost, vcs.CreateInitialPREndpoint, gh.CreateInitialPR)
}

func (gh *ghAppService) getUsernameMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			sessionID, err := c.Cookie("session_id")
			if err != nil {
				err = c.JSON(http.StatusNotAcceptable, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
				return err
			}
			userID := strings.Split(sessionID.Value, ":")[1]
			user, err := gh.store.GetUserById(c.Request().Context(), userID, false, nil)
			if err != nil {
				err = c.JSON(http.StatusNotAcceptable, echo.Map{
					"error": err.Error(),
				})
				gh.logger.Log(c, err).Send()
				return err
			}

			c.Set("username", user.Username)
			return next(c)
		}
	}
}

func newGHClient(appID int64, privKeyPem string) (*ghinstallation.AppsTransport, *github.Client, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(
		http.DefaultTransport,
		appID,
		privKeyPem,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("ERR_CREATE_NEW_TRANSPORT: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport, Timeout: time.Second * 30})
	return transport, client, nil
}

func (gh *ghAppService) refreshGHClient(appTransport *ghinstallation.AppsTransport, id int64) *github.Client {
	transport := ghinstallation.NewFromAppsTransport(appTransport, id)
	return github.NewClient(&http.Client{Transport: transport})
}

type AuthorizedRepository struct {
	Repository *github.Repository `json:"repository"`
	Branches   []*github.Branch   `json:"branches"`
}

const WorkflowFilePath = ".github/workflows/openregistry-build.yml"
const OpenRegistryAutomationBranchName = "openregistry-build-automation"
