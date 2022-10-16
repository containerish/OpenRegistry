package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/containerish/OpenRegistry/vcs"
	"github.com/google/go-github/v46/github"
	"github.com/labstack/echo/v4"
)

func (gh *ghAppService) HandleAppFinish(ctx echo.Context) error {
	username := ctx.Get(UsernameContextKey).(string)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err = gh.store.UpdateInstallationID(ctx.Request().Context(), installationID, username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.Redirect(http.StatusTemporaryRedirect, gh.config.AppInstallRedirectURL)
	gh.logger.Log(ctx, echoErr).Send()
	return echoErr
}

// HandleSetupCallback implements vcs.VCS
func (gh *ghAppService) HandleSetupCallback(ctx echo.Context) error {
	username := ctx.Get(UsernameContextKey).(string)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err := gh.store.UpdateInstallationID(ctx.Request().Context(), installationID, username); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.Redirect(http.StatusTemporaryRedirect, gh.config.AppInstallRedirectURL)
	gh.logger.Log(ctx, echoErr).Send()
	return echoErr
}

// HandleWebhookEvents implements vcs.VCS
func (gh *ghAppService) HandleWebhookEvents(ctx echo.Context) error {
	return ctx.NoContent(http.StatusNoContent)
}

// ListAuthorisedRepositories implements vcs.VCS
func (gh *ghAppService) ListAuthorisedRepositories(ctx echo.Context) error {
	installationID := ctx.Get(GithubInstallationIDContextKey).(int64)

	client := gh.refreshGHClient(gh.ghAppTransport, installationID)
	repos, _, err := client.Apps.ListRepos(context.Background(), &github.ListOptions{})
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
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
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			gh.logger.Log(ctx, err).Send()
			return echoErr
		}

		sort.Slice(b, func(i, j int) bool {
			return b[i].GetName() == repo.GetDefaultBranch()
		})

		repoList = append(repoList, &AuthorizedRepository{
			Repository: repo,
			Branches:   b,
		})
	}

	err = ctx.JSON(http.StatusOK, repoList)
	gh.logger.Log(ctx, err).Send()
	return err
}

func (gh *ghAppService) CreateInitialPR(ctx echo.Context) error {
	installationID := ctx.Get(GithubInstallationIDContextKey).(int64)

	var req vcs.InitialPRRequest
	if err := json.NewDecoder(ctx.Request().Body).Decode(&req); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	client := gh.refreshGHClient(gh.ghAppTransport, installationID)
	repos, _, err := client.Apps.ListRepos(context.Background(), &github.ListOptions{})
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	var repository github.Repository
	for _, r := range repos.Repositories {
		if r.GetName() == req.RepositoryName {
			repository = *r
			break
		}
	}

	if repository.Name == nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "repository not found in authorized repository list",
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	workflowExists := gh.doesWorkflowExist(ctx.Request().Context(), client, &repository)
	if workflowExists {
		echoErr := ctx.JSON(http.StatusNotModified, echo.Map{
			"error": "workflow file already exists",
		})
		gh.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	if err = gh.createGitubActionsWorkflow(ctx.Request().Context(), client, &repository); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	prTemplate, err := gh.populateInitialPRTempplate()
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"code":  "CREATE_WORKFLOW",
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	opts := &github.NewPullRequest{
		Title:               github.String("build(ci): OpenRegistry build and push"),
		Base:                github.String(repository.GetDefaultBranch()),
		Head:                github.String(gh.automationBranchName),
		Body:                github.String(prTemplate),
		MaintainerCanModify: github.Bool(true),
	}

	_, _, err = client.PullRequests.Create(
		ctx.Request().Context(),
		repository.GetOwner().GetLogin(),
		req.RepositoryName,
		opts,
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"code":  "CREATE_WORKFLOW",
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusCreated, echo.Map{
		"message": "Pull request created successfully",
	})
	gh.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (gh *ghAppService) populateInitialPRTempplate() (string, error) {
	tpl, err := template.New("github-pull-request").Parse(InitialPRBody)
	if err != nil {
		return "", fmt.Errorf("ERR_TEMPLATE_PARSE: %w", err)
	}

	buf := &bytes.Buffer{}
	td := InitialPRTemplateData{
		WebInterfaceURL: gh.webInterfaceURL,
	}

	if err = tpl.Execute(buf, td); err != nil {
		return "", fmt.Errorf("ERR_TEMPLATE_EXEC: %w", err)
	}

	return buf.String(), nil
}

func (gh *ghAppService) createGitubActionsWorkflow(
	ctx context.Context,
	client *github.Client,
	repo *github.Repository,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	err := gh.createBranch(ctx, client, repo)
	if err != nil {
		return err
	}

	return gh.createWorkflowFile(ctx, client, repo)
}
