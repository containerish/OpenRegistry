package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/google/go-github/v56/github"
	"gopkg.in/yaml.v3"
)

func (gh *ghAppService) doesWorkflowExist(ctx context.Context, client *github.Client, r *github.Repository) bool {
	branches := []string{r.GetDefaultBranch(), gh.automationBranchName}

	for _, b := range branches {
		childCtx, cancel := context.WithTimeout(ctx, time.Second*10)

		opts := &github.RepositoryContentGetOptions{Ref: b}
		_, _, _, err := client.Repositories.GetContents(
			childCtx,
			r.GetOwner().GetLogin(),
			r.GetName(),
			gh.workflowFilePath,
			opts,
		)
		cancel()
		if err == nil {
			return true
		}

	}

	return false
}

func (gh *ghAppService) createBranch(ctx context.Context, client *github.Client, r *github.Repository) error {
	base, _, err := client.Repositories.GetBranch(
		ctx,
		r.GetOwner().GetLogin(),
		r.GetName(),
		r.GetDefaultBranch(),
		MaxGitHubRedirects,
	)
	if err != nil {
		return fmt.Errorf("ERR_GET_BRANCH: %w", err)
	}

	baseBranchSha := base.GetCommit().GetSHA()
	newBranchRefName := fmt.Sprintf("refs/heads/%s", gh.automationBranchName)

	ref := &github.Reference{
		Ref: github.String(newBranchRefName),
		Object: &github.GitObject{
			SHA: &baseBranchSha,
		},
	}

	_, resp, err := client.Git.CreateRef(ctx, r.GetOwner().GetLogin(), r.GetName(), ref)

	// if the branch already exists, all is good?
	if resp.StatusCode == http.StatusUnprocessableEntity {
		// branch exists
		return nil
	}

	if err != nil {
		return fmt.Errorf("ERR_CREATE_REF: %w", err)
	}

	return nil
}

type WorkflowProperties struct {
	RegistryEndpoint string
	RepositoryOwner  string
	RepositoryName   string
}

func (gh *ghAppService) createWorkflowFile(
	ctx context.Context,
	client *github.Client,
	r *github.Repository,
	props *WorkflowProperties,
) error {
	msg := "build(ci): OpenRegistry build and push"

	tpl, err := template.New("github-actions-workflow").Delims("[[", "]]").Parse(buildAndPushTemplate)
	if err != nil {
		return fmt.Errorf("TEMPLATE_ERR: %w", err)
	}

	buf := &bytes.Buffer{}
	if err = tpl.Execute(buf, props); err != nil {
		return fmt.Errorf("ERR_EXECUTE_TEMPLATE: %w", err)
	}

	yamlWorkflow := make(map[any]any)
	if err = yaml.Unmarshal(buf.Bytes(), &yamlWorkflow); err != nil {
		return fmt.Errorf("ERR_ENCODE_YAML_WORKFLOW: %w", err)
	}

	yamlWorkflowBz, err := yaml.Marshal(yamlWorkflow)
	if err != nil {
		return err
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.String(msg),
		Content: yamlWorkflowBz,
		Branch:  github.String(gh.automationBranchName),
	}
	_, _, err = client.Repositories.CreateFile(ctx, r.GetOwner().GetLogin(), r.GetName(), gh.workflowFilePath, opts)
	if err != nil {
		return fmt.Errorf("ERR_CREATE_WORKFLPW_FILE: %w", err)
	}

	return nil
}
