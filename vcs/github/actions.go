package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/google/go-github/v46/github"
)

func (gh *ghAppService) doesWorkflowExist(ctx context.Context, client *github.Client, owner, repo, branch string) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	opts := &github.RepositoryContentGetOptions{
		Ref: branch,
	}
	_, _, _, err := client.Repositories.GetContents(ctx, owner, repo, gh.workflowFilePath, opts)

	return err == nil
}

func (gh *ghAppService) createBranch(ctx context.Context, client *github.Client, owner, repo, baseBranch, branch string) (bool, error) {
	base, _, err := client.Repositories.GetBranch(ctx, owner, repo, baseBranch, true)
	if err != nil {
		return false, fmt.Errorf("ERR_GET_BRANCH: %w", err)
	}

	baseBranchSha := base.GetCommit().GetSHA()
	newBranchRefName := fmt.Sprintf("refs/heads/%s", branch)

	ref := &github.Reference{
		Ref: github.String(newBranchRefName),
		Object: &github.GitObject{
			SHA: &baseBranchSha,
		},
	}

	_, resp, err := client.Git.CreateRef(ctx, owner, repo, ref)

	// if the branch already exists, all is good?
	if resp.StatusCode == http.StatusUnprocessableEntity {
		// branch exists
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("ERR_CREATE_REF: %w", err)
	}

	return false, nil
}

func (gh *ghAppService) createWorkflowFile(ctx context.Context, client *github.Client, owner, repo, branch, mainBranch string) error {
	msg := "build(ci): OpenRegistry build and push"

	tpl, err := template.New("github-actions-workflow").Delims("[[", "]]").Parse(buildAndPushTemplate)
	if err != nil {
		return fmt.Errorf("TEMPLATE_ERR: %w", err)
	}
	buf := &bytes.Buffer{}

	if err = tpl.Execute(buf, mainBranch); err != nil {
		return fmt.Errorf("ERR_EXEC_TEMPLATE: %w", err)
	}

	if _, _, err = client.Repositories.CreateFile(
		context.Background(),
		owner,
		repo,
		gh.workflowFilePath,
		&github.RepositoryContentFileOptions{
			Message: github.String(msg),
			Content: buf.Bytes(),
			Branch:  &branch,
		},
	); err != nil {
		return fmt.Errorf("ERR_CREATE_WORKFLPW_FILE: %w", err)
	}

	return nil
}
