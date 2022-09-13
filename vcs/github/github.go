package github

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/google/go-github/v46/github"
	"github.com/labstack/echo/v4"
)

type ghAppService struct {
	config *config.Integation
	store  vcs.VCSStore
}

type AuthorizedRepository struct {
	Repository *github.Repository `json:"repository"`
	Branches   []*github.Branch   `json:"branches"`
}

func NewGithubApp(cfg *config.Integation, store vcs.VCSStore) (vcs.VCS, error) {
	return &ghAppService{
		config: cfg,
		store:  store,
	}, nil
}

func (gh *ghAppService) ListHandlers() []echo.HandlerFunc {
	return []echo.HandlerFunc{
		gh.HandleSetupCallback,
		gh.HandleWebhookEvents,
		gh.ListAuthorisedRepositories,
		gh.HandleGithubAppFinish,
	}
}

func (gh *ghAppService) RegisterRoutes(r *echo.Group) {
	r.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			sessionID, err := c.Cookie("session_id")
			if err != nil {
				return c.JSON(http.StatusBadRequest, echo.Map{
					"error": err.Error(),
				})
			}
			userID := strings.Split(sessionID.Value, ":")[1]
			user, err := gh.store.GetUserById(c.Request().Context(), userID, false, nil)
			if err != nil {
				return c.JSON(http.StatusBadRequest, echo.Map{
					"error": err.Error(),
				})
			}

			c.Set("username", user.Username)
			return next(c)
		}
	})
	r.Add(http.MethodGet, "/app/repo/list", gh.ListAuthorisedRepositories)
	r.Add(http.MethodGet, "/app/callback", gh.HandleSetupCallback)
	r.Add(http.MethodPost, "/app/webhooks/listen", gh.HandleWebhookEvents)
	r.Add(http.MethodPost, "/app/setup/finish", gh.HandleGithubAppFinish)
}

func (gh *ghAppService) HandleGithubAppFinish(ctx echo.Context) error {
	ghAppInstallationID := ctx.QueryParam("installation_id")
	username := ctx.Get("username").(string)

	if err := gh.store.UpdateInstallationID(ctx.Request().Context(), ghAppInstallationID, username); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return ctx.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000/apps/github/connect/select-repo")
}

// HandleSetupCallback implements vcs.VCS
func (gh *ghAppService) HandleSetupCallback(ctx echo.Context) error {
	ghAppInstallationID := ctx.QueryParam("installation_id")

	if err := gh.store.UpdateInstallationID(ctx.Request().Context(), ghAppInstallationID, ""); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return ctx.Redirect(http.StatusTemporaryRedirect, "http://localhost:3000/apps/github/connect/select-repo")
}

// HandleWebhookEvents implements vcs.VCS
func (gh *ghAppService) HandleWebhookEvents(ctx echo.Context) error {
	return ctx.NoContent(http.StatusNoContent)
}

// ListAuthorisedRepositories implements vcs.VCS
func (gh *ghAppService) ListAuthorisedRepositories(ctx echo.Context) error {
	transport, _, err := gh.newGHClient()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	username := ctx.Get("username").(string)
	installationID, err := gh.store.GetInstallationID(ctx.Request().Context(), username)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	id, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	client := gh.refreshGHClient(transport, id)
	repos, _, err := client.Apps.ListRepos(context.Background(), &github.ListOptions{})
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	repoList := make([]*AuthorizedRepository, 0)
	for _, repo := range repos.Repositories {
		b, _, err := client.Repositories.ListBranches(
			ctx.Request().Context(),
			repo.GetOwner().GetLogin(),
			repo.GetName(),
			&github.BranchListOptions{},
		)
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		sort.Slice(b, func(i, j int) bool {
			return b[i].GetName() == repo.GetDefaultBranch()
		})

		repoList = append(repoList, &AuthorizedRepository{
			Repository: repo,
			Branches:   b,
		})
	}

	return ctx.JSON(http.StatusOK, repoList)
}

func (gh *ghAppService) newGHClient() (*ghinstallation.AppsTransport, *github.Client, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(
		http.DefaultTransport,
		gh.config.AppID,
		"github-app.pem",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("ERR_CREATE_NEW_TRANSPORT: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport, Timeout: time.Second * 30})
	return transport, client, nil
}

func (gh *ghAppService) refreshGHClient(appTransport *ghinstallation.AppsTransport, installationId int64) *github.Client {
	transport := ghinstallation.NewFromAppsTransport(appTransport, installationId)
	return github.NewClient(&http.Client{Transport: transport})
}
