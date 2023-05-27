package build_automation_store

import (
	"context"
	"fmt"

	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
)

// DeleteProject implements BuildAutomationStore
func (p *pg) DeleteProject(ctx context.Context, project *github_actions_v1.DeleteProjectRequest) error {
	query := `delete from build_projects where id=$1`
	_, err := p.conn.Exec(ctx, query, project.GetId())
	if err != nil {
		return fmt.Errorf("ERR_DELETE_PROJECT: %w", err)
	}

	return nil
}

// GetProject implements BuildAutomationStore
func (p *pg) GetProject(ctx context.Context, project *github_actions_v1.GetProjectRequest) (*github_actions_v1.GetProjectResponse, error) {
	query := `select
    id,
    name,
    production_branch,
    created_at,
    build_tool,
    exec_command,
    workflow_file,
    environment_variables
    from build_projects where id=$1`
	row := p.conn.QueryRow(ctx, query, project.GetId())

	proj := &github_actions_v1.GetProjectResponse{}
	proj.BuildSettings = &github_actions_v1.ProjectBuildSettingsMessage{}
	err := row.Scan(
		&proj.Id,
		&proj.ProjectName,
		&proj.ProductionBranch,
		&proj.CreatedAt,
		&proj.BuildSettings.BuildTool,
		&proj.BuildSettings.ExecCommand,
		&proj.BuildSettings.WorfklowFile,
		&proj.EnvironmentVariables,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_PROJECTS_SCAN: %w", err)
	}

	return proj, nil
}

// ListProjects implements BuildAutomationStore
func (p *pg) ListProjects(ctx context.Context, project *github_actions_v1.ListProjectsRequest) (*github_actions_v1.ListProjectsResponse, error) {
	query := `select
    id,
    name,
    production_branch,
    created_at,
    build_tool,
    exec_command,
    workflow_file,
    environment_variables
    from build_projects where owner=$1`
	rows, err := p.conn.Query(ctx, query, project.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("ERR_LIST_PROJECTS: %w", err)
	}

	projects := &github_actions_v1.ListProjectsResponse{}
	for rows.Next() {
		proj := &github_actions_v1.GetProjectResponse{}
		proj.BuildSettings = &github_actions_v1.ProjectBuildSettingsMessage{}
		err = rows.Scan(
			&proj.Id,
			&proj.ProjectName,
			&proj.ProductionBranch,
			&proj.CreatedAt,
			&proj.BuildSettings.BuildTool,
			&proj.BuildSettings.ExecCommand,
			&proj.BuildSettings.WorfklowFile,
			&proj.EnvironmentVariables,
		)
		if err != nil {
			return nil, fmt.Errorf("ERR_LIST_PROJECTS_SCAN: %w", err)
		}

		projects.Projects = append(projects.Projects, proj)
	}

	return projects, nil
}

// StoreProject implements BuildAutomationStore
func (p *pg) StoreProject(ctx context.Context, project *github_actions_v1.CreateProjectRequest) error {
	query := `insert into build_projects
    (id, name, owner, production_branch, created_at, build_tool, exec_command, workflow_file, environment_variables)
    values($1,$2,$3,$4,$5,$6,$7,$8,$9)
    `
	_, err := p.conn.Exec(
		ctx,
		query,
		project.GetId(),
		project.GetProjectName(),
		project.GetOwner(),
		project.GetProductionBranch(),
		project.GetCreatedAt(),
		project.GetBuildSettings().GetBuildTool(),
		project.GetBuildSettings().GetExecCommand(),
		project.GetBuildSettings().GetWorfklowFile(),
		project.GetEnvironmentVariables(),
	)
	if err != nil {
		return fmt.Errorf("ERR_STORE_PROJECT: %w", err)
	}

	return nil
}

func (p *pg) Close() {
	p.conn.Close()
}
