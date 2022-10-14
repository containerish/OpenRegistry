package github

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/containerish/OpenRegistry/vcs"
	"github.com/google/go-github/v46/github"
	"github.com/labstack/echo/v4"
)

func (gh *ghAppService) HandleAppFinish(ctx echo.Context) error {
	username := ctx.Get("username").(string)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	if err := gh.store.UpdateInstallationID(ctx.Request().Context(), installationID, username); err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	return ctx.Redirect(http.StatusTemporaryRedirect, gh.config.AppInstallRedirectURL)
}

// HandleSetupCallback implements vcs.VCS
func (gh *ghAppService) HandleSetupCallback(ctx echo.Context) error {
	username := ctx.Get("username").(string)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	if err := gh.store.UpdateInstallationID(ctx.Request().Context(), installationID, username); err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	return ctx.Redirect(http.StatusTemporaryRedirect, gh.config.AppInstallRedirectURL)
}

// HandleWebhookEvents implements vcs.VCS
func (gh *ghAppService) HandleWebhookEvents(ctx echo.Context) error {
	return ctx.NoContent(http.StatusNoContent)
}

// ListAuthorisedRepositories implements vcs.VCS
func (gh *ghAppService) ListAuthorisedRepositories(ctx echo.Context) error {
	username := ctx.Get("username").(string)
	installationID, err := gh.store.GetInstallationID(ctx.Request().Context(), username)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	client := gh.refreshGHClient(gh.ghAppTransport, installationID)
	repos, _, err := client.Apps.ListRepos(context.Background(), &github.ListOptions{})
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
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
			err = ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			gh.logger.Log(ctx, err).Send()
			return err
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

func (gh *ghAppService) CreateInitialPR(ctx echo.Context) error {
	username := ctx.Get("username").(string)

	installationID, err := gh.store.GetInstallationID(ctx.Request().Context(), username)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	var req vcs.InitialPRRequest
	if err = json.NewDecoder(ctx.Request().Body).Decode(&req); err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	client := gh.refreshGHClient(gh.ghAppTransport, installationID)
	repos, _, err := client.Apps.ListRepos(context.Background(), &github.ListOptions{})
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	var repository github.Repository
	for _, r := range repos.Repositories {
		if r.GetName() == req.RepositoryName {
			repository = *r
			break
		}
	}

	if repository.Name == nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "repository not found in authorized repository list",
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	workflowExists := gh.doesWorkflowExist(
		ctx.Request().Context(),
		client,
		repository.Owner.GetLogin(),
		req.RepositoryName,
		repository.GetDefaultBranch(),
		gh.automationBranchName,
	)
	if workflowExists {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"code":  "DOES_WORKFLOW_EXIST",
			"error": "file already exists",
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	err = gh.createBranch(
		ctx.Request().Context(),
		client,
		repository.Owner.GetLogin(),
		req.RepositoryName,
		repository.GetDefaultBranch(),
		gh.automationBranchName,
	)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	err = gh.createWorkflowFile(
		ctx.Request().Context(),
		client, repository.GetOwner().GetLogin(),
		req.RepositoryName,
		gh.automationBranchName,
		repository.GetDefaultBranch(),
	)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"code":  "CREATE_WORKFLOW",
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return err
	}

	opts := &github.NewPullRequest{
		Title:               github.String("build(ci): OpenRegistry build and push"),
		Base:                github.String(repository.GetDefaultBranch()),
		Head:                github.String(gh.automationBranchName),
		Body:                github.String(InitialPRBody),
		MaintainerCanModify: github.Bool(true),
	}

	_, _, err = client.PullRequests.Create(
		ctx.Request().Context(),
		repository.GetOwner().GetLogin(),
		req.RepositoryName,
		opts,
	)
	if err != nil {
		err = ctx.JSON(http.StatusBadRequest, echo.Map{
			"code":  "CREATE_WORKFLOW",
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
	}

	err = ctx.JSON(http.StatusCreated, echo.Map{
		"message": "Pull request created successfully",
	})

	gh.logger.Log(ctx, err).Send()
	return err
}
