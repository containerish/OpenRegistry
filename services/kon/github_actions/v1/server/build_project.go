package server

import (
	"context"

	connect_go "github.com/bufbuild/connect-go"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
)

// CreateProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) CreateProject(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.CreateProjectRequest],
) (
	*connect_go.Response[github_actions_v1.CreateProjectResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	if err = ghs.store.StoreProject(ctx, req.Msg); err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(&github_actions_v1.CreateProjectResponse{
		Message: "project created successfully",
	})
	return resp, nil
}

// DeleteProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) DeleteProject(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.DeleteProjectRequest],
) (
	*connect_go.Response[github_actions_v1.DeleteProjectResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	if err = ghs.store.DeleteProject(ctx, req.Msg); err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(&github_actions_v1.DeleteProjectResponse{
		Message: "project deleted successfully",
	})

	return resp, nil
}

// GetProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) GetProject(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.GetProjectRequest],
) (
	*connect_go.Response[github_actions_v1.GetProjectResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	project, err := ghs.store.GetProject(ctx, req.Msg)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(project)
	return resp, nil
}

// ListProjects implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) ListProjects(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.ListProjectsRequest],
) (
	*connect_go.Response[github_actions_v1.ListProjectsResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	projects, err := ghs.store.ListProjects(ctx, req.Msg)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(projects)
	return resp, nil
}
