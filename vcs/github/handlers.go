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

	"github.com/google/go-github/v56/github"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/vcs"
)

func (gh *ghAppService) HandleAppFinish(ctx echo.Context) error {
	user := ctx.Get(string(types.UserContextKey)).(*types.User)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	installation, _, err := gh.ghClient.Apps.GetInstallation(ctx.Request().Context(), installationID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not find the GitHub App installation",
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	ghClient := gh.refreshGHClient(installationID)
	ghUser, _, err := ghClient.Users.Get(ctx.Request().Context(), installation.GetAccount().GetLogin())
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not find the GitHub App installation",
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	if user.Identities == nil {
		user.Identities = types.Identities{}
	}

	if user.Identities.GetGitHubIdentity() == nil {
		user.Identities[types.IdentityProviderGitHub] = &types.UserIdentity{
			ID:             fmt.Sprintf("%d", ghUser.GetID()),
			Name:           ghUser.GetName(),
			Username:       ghUser.GetLogin(),
			Email:          ghUser.GetEmail(),
			Avatar:         ghUser.GetAvatarURL(),
			InstallationID: installationID,
		}
	} else {
		user.Identities[types.IdentityProviderGitHub].Username = ghUser.GetEmail()
		user.Identities[types.IdentityProviderGitHub].Email = ghUser.GetLogin()
	}

	if _, err = gh.store.UpdateUser(ctx.Request().Context(), user); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.NoContent(http.StatusAccepted)
	gh.logger.Log(ctx, echoErr).Send()
	return echoErr
}

// HandleSetupCallback implements vcs.VCS
func (gh *ghAppService) HandleSetupCallback(ctx echo.Context) error {
	user := ctx.Get(string(types.UserContextKey)).(*types.User)

	installationID, err := strconv.ParseInt(ctx.QueryParam("installation_id"), 10, 64)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	if user.Identities == nil {
		user.Identities = types.Identities{}
	}

	if user.Identities.GetGitHubIdentity() == nil {
		user.Identities[types.IdentityProviderGitHub] = &types.UserIdentity{}
	}

	installation, _, err := gh.ghClient.Apps.GetInstallation(ctx.Request().Context(), installationID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not find the GitHub App installation",
		})

		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	ghClient := gh.refreshGHClient(installationID)
	ghUser, _, err := ghClient.Users.Get(ctx.Request().Context(), installation.GetAccount().GetLogin())
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})

		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	user.Identities[types.IdentityProviderGitHub] = &types.UserIdentity{
		ID:             fmt.Sprintf("%d", installation.GetID()),
		Name:           ghUser.GetName(),
		Username:       ghUser.GetLogin(),
		Email:          ghUser.GetEmail(),
		Avatar:         ghUser.GetAvatarURL(),
		InstallationID: installationID,
	}

	if _, err = gh.store.UpdateUser(ctx.Request().Context(), user); err != nil {
		// if err := gh.store.UpdateInstallationID(ctx.Request().Context(), installationID, username); err != nil {
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
	xHubSignature := ctx.Request().Header.Get("X-Hub-Signature-256")

	payload, err := github.ValidatePayloadFromBody(
		ctx.Request().Header.Get("Content-Type"),
		ctx.Request().Body,
		xHubSignature,
		[]byte(gh.config.WebhookSecret),
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(ctx.Request()), payload)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	switch event := event.(type) {
	case *github.PingEvent:
	case *github.WorkflowJobEvent:
	case *github.WorkflowRunEvent:
		gh.handleWorkflowRunEvents(event)
	case *github.WorkflowDispatchEvent:
	case *github.InstallationRepositoriesEvent:
	case *github.CheckRunEvent:
	case *github.InstallationEvent:
	}

	echoErr := ctx.NoContent(http.StatusNoContent)
	gh.logger.Log(ctx, nil).Send()
	return echoErr
}

// @TODO pending implementation (@jay-dee7)
func (gh *ghAppService) handleWorkflowRunEvents(event *github.WorkflowRunEvent) {
	client := gh.refreshGHClient(event.GetInstallation().GetID())
	repo := event.GetRepo()
	if event.GetAction() == "in_progress" || event.GetAction() == "completed" {
		logsUrl, resp, err := client.Actions.GetWorkflowRunLogs(
			context.Background(),
			repo.GetOwner().GetLogin(),
			repo.GetName(),
			event.GetWorkflowRun().GetID(),
			MaxGitHubRedirects,
		)
		if err != nil {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(resp.Body)
			resp.Body.Close()
			return
		}

		var logs []byte
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, logsUrl.String(), nil)
		_, err = client.Do(context.Background(), req, &logs)
		if err != nil {
			return
		}
	}
}

// ListAuthorisedRepositories implements vcs.VCS
func (gh *ghAppService) ListAuthorisedRepositories(ctx echo.Context) error {
	installationID := ctx.Get(string(GithubInstallationIDContextKey)).(int64)

	client := gh.refreshGHClient(installationID)
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
		branches, _, bErr := client.Repositories.ListBranches(
			ctx.Request().Context(),
			repo.GetOwner().GetLogin(),
			repo.GetName(),
			&github.BranchListOptions{},
		)
		if bErr != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": bErr.Error(),
			})
			gh.logger.Log(ctx, bErr).Send()
			return echoErr
		}

		sort.Slice(branches, func(i, j int) bool {
			return branches[i].GetName() == repo.GetDefaultBranch()
		})

		repoList = append(repoList, &AuthorizedRepository{
			Repository: repo,
			Branches:   branches,
		})
	}

	echoErr := ctx.JSON(http.StatusOK, repoList)
	gh.logger.Log(ctx, nil).Send()
	return echoErr
}

func (gh *ghAppService) CreateInitialPR(ctx echo.Context) error {
	installationID := ctx.Get(string(GithubInstallationIDContextKey)).(int64)
	user := ctx.Get(string(types.UserContextKey)).(*types.User)

	var req vcs.InitialPRRequest
	if err := json.NewDecoder(ctx.Request().Body).Decode(&req); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	client := gh.refreshGHClient(installationID)
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
		echoErr := ctx.NoContent(http.StatusNoContent)
		gh.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	err = gh.createGitubActionsWorkflow(
		ctx.Request().Context(),
		client,
		&repository,
		&WorkflowProperties{
			RegistryEndpoint: gh.registryEndpoint,
			RepositoryOwner:  user.Username,
			RepositoryName:   req.RepositoryName,
		},
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		gh.logger.Log(ctx, err).Send()
		return echoErr
	}

	webAppURL := gh.config.GetAllowedURLFromEchoContext(ctx, gh.env, gh.webInterfaceURLs)
	prTemplate, err := gh.populateInitialPRTempplate(webAppURL)
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

func (gh *ghAppService) populateInitialPRTempplate(webAppURL string) (string, error) {
	tpl, err := template.New("github-pull-request").Parse(InitialPRBody)
	if err != nil {
		return "", fmt.Errorf("ERR_TEMPLATE_PARSE: %w", err)
	}

	buf := &bytes.Buffer{}
	td := InitialPRTemplateData{
		WebInterfaceURL: webAppURL,
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
	props *WorkflowProperties,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	err := gh.createBranch(ctx, client, repo)
	if err != nil {
		return err
	}

	return gh.createWorkflowFile(ctx, client, repo, props)
}
