package server

import (
	"context"
	"time"

	connect_go "github.com/bufbuild/connect-go"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/vcs/github"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) CreateProject(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.CreateProjectRequest],
) (
	*connect_go.Response[github_actions_v1.CreateProjectResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	if err = req.Msg.GetCreatedAt().CheckValid(); err != nil {
		req.Msg.CreatedAt = timestamppb.New(time.Now())
	}

	if err = ghs.store.StoreProject(ctx, req.Msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(&github_actions_v1.CreateProjectResponse{
		Message: "project created successfully",
	})

	logEvent.Bool("success", true).Send()
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
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	if err = ghs.store.DeleteProject(ctx, req.Msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(&github_actions_v1.DeleteProjectResponse{
		Message: "project deleted successfully",
	})

	logEvent.Bool("success", true).Send()
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
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	if err := req.Msg.Validate(); err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	project, err := ghs.store.GetProject(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()
	return connect_go.NewResponse(project), nil
}

// ListProjects implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) ListProjects(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.ListProjectsRequest],
) (
	*connect_go.Response[github_actions_v1.ListProjectsResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	user := ctx.Value(github.UserContextKey).(*types.User)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	req.Msg.UserId = user.ID.String()

	projects, err := ghs.store.ListProjects(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()
	return connect_go.NewResponse(projects), nil
}
