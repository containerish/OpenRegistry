package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	common_v1 "github.com/containerish/OpenRegistry/common/v1"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	v1 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

// DeleteProject implements BuildAutomationStore
func (s *store) DeleteProject(ctx context.Context, project *github_actions_v1.DeleteProjectRequest) error {
	if _, err := s.
		db.
		NewDelete().
		Model(&types.RepositoryBuildProject{}).
		Where("id = ?", project.GetId().GetValue()).
		Exec(ctx); err != nil {
		return fmt.Errorf("ERR_DELETE_PROJECT: %w", err)
	}

	return nil
}

// GetProject implements BuildAutomationStore
func (s *store) GetProject(
	ctx context.Context,
	project *github_actions_v1.GetProjectRequest,
) (*github_actions_v1.GetProjectResponse, error) {
	var proj types.RepositoryBuildProject
	if err := s.db.NewSelect().Model(&proj).WherePK().Scan(ctx); err != nil {
		return nil, fmt.Errorf("ERR_GET_PROJECTS_SCAN: %w", err)
	}
	protoProj := &github_actions_v1.GetProjectResponse{
		Id: &common_v1.UUID{
			Value: proj.ID.String(),
		},
		ProjectName:      proj.Name,
		ProductionBranch: proj.ProductionBranch,
		BuildSettings: &github_actions_v1.ProjectBuildSettingsMessage{
			BuildTool:    proj.BuildTool,
			ExecCommand:  proj.ExecCommand,
			WorfklowFile: proj.WorkflowFile,
		},
		CreatedAt: timestamppb.New(proj.CreatedAt),
	}

	for key, value := range proj.EnvironmentVariables {
		protoProj.EnvironmentVariables.EnvironmentVariables = append(
			protoProj.EnvironmentVariables.EnvironmentVariables,
			&github_actions_v1.ProjectEnvironmentVariable{
				Key:   key,
				Value: value,
			})
	}

	return protoProj, nil
}

// ListProjects implements BuildAutomationStore
func (s *store) ListProjects(
	ctx context.Context,
	req *github_actions_v1.ListProjectsRequest,
) (*github_actions_v1.ListProjectsResponse, error) {
	projects := make([]*types.RepositoryBuildProject, 0)

	err := s.
		db.
		NewSelect().
		Model(&projects).
		Relation("Repository").
		Where("owner_id = ?", req.GetOwnerId().GetValue()).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("ERR_LIST_PROJECTS: %w", err)
	}

	protoProjects := &github_actions_v1.ListProjectsResponse{
		Projects: make([]*github_actions_v1.GetProjectResponse, len(projects)),
	}

	for i, p := range projects {
		proj := &github_actions_v1.GetProjectResponse{
			Id: &common_v1.UUID{
				Value: p.ID.String(),
			},
			ProjectName:      p.Name,
			ProductionBranch: p.ProductionBranch,
			BuildSettings: &github_actions_v1.ProjectBuildSettingsMessage{
				BuildTool:    p.BuildTool,
				ExecCommand:  p.ExecCommand,
				WorfklowFile: p.WorkflowFile,
			},
			CreatedAt: timestamppb.New(p.CreatedAt),
			OwnerId: &common_v1.UUID{
				Value: p.RepositoryOwnerID.String(),
			},
			RepositoryId: &common_v1.UUID{
				Value: p.RepositoryID.String(),
			},
			RepositoryName: p.Repository.Name,
		}

		for key, value := range p.EnvironmentVariables {
			proj.EnvironmentVariables.EnvironmentVariables = append(
				proj.EnvironmentVariables.EnvironmentVariables,
				&github_actions_v1.ProjectEnvironmentVariable{
					Key:       key,
					Value:     value,
					Encrypted: false,
				},
			)
		}

		protoProjects.Projects[i] = proj
	}

	return protoProjects, nil
}

// StoreProject implements BuildAutomationStore
func (s *store) StoreProject(ctx context.Context, project *github_actions_v1.CreateProjectRequest) error {
	projectID, err := uuid.Parse(project.GetId().GetValue())
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}
	ownerID, err := uuid.Parse(project.GetOwnerId().GetValue())
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}

	repositoryID, err := uuid.Parse(project.GetRepositoryId().GetValue())
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}

	proj := &types.RepositoryBuildProject{
		CreatedAt:         time.Now(),
		Name:              project.GetProjectName(),
		ProductionBranch:  project.GetProductionBranch(),
		BuildTool:         project.GetBuildSettings().GetBuildTool(),
		ExecCommand:       project.GetBuildSettings().GetExecCommand(),
		WorkflowFile:      project.GetBuildSettings().GetWorfklowFile(),
		ID:                projectID,
		RepositoryOwnerID: ownerID,
		RepositoryID:      repositoryID,
	}

	for _, envVar := range project.GetEnvironmentVariables().GetEnvironmentVariables() {
		proj.EnvironmentVariables[envVar.GetKey()] = envVar.GetValue()
	}

	if _, err := s.db.NewInsert().Model(proj).Exec(ctx); err != nil {
		return fmt.Errorf("ERR_STORE_PROJECT: %w", err)
	}

	return nil
}
