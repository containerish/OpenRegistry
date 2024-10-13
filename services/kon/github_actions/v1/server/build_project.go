package server

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	common_v1 "github.com/containerish/OpenRegistry/common/v1"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

// CreateProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) CreateProject(
	ctx context.Context,
	req *connect.Request[github_actions_v1.CreateProjectRequest],
) (
	*connect.Response[github_actions_v1.CreateProjectResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	req.Msg.Id = &common_v1.UUID{
		Value: uuid.New().String(),
	}
	req.Msg.CreatedAt = timestamppb.Now()
	if err = ghs.store.StoreProject(ctx, req.Msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := connect.NewResponse(&github_actions_v1.CreateProjectResponse{
		Message: "project created successfully",
		Id:      req.Msg.GetId(),
	})

	logEvent.Bool("success", true).Send()
	return resp, nil
}

// DeleteProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) DeleteProject(
	ctx context.Context,
	req *connect.Request[github_actions_v1.DeleteProjectRequest],
) (
	*connect.Response[github_actions_v1.DeleteProjectResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err = ghs.store.DeleteProject(ctx, req.Msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := connect.NewResponse(&github_actions_v1.DeleteProjectResponse{
		Message: "project deleted successfully",
	})

	logEvent.Bool("success", true).Send()
	return resp, nil
}

// GetProject implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) GetProject(
	ctx context.Context,
	req *connect.Request[github_actions_v1.GetProjectRequest],
) (
	*connect.Response[github_actions_v1.GetProjectResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	if err := req.Msg.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	project, err := ghs.store.GetProject(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()
	return connect.NewResponse(project), nil
}

// ListProjects implements github_actions_v1connect.GitHubActionsProjectServiceHandler
func (ghs *GitHubActionsServer) ListProjects(
	ctx context.Context,
	req *connect.Request[github_actions_v1.ListProjectsRequest],
) (
	*connect.Response[github_actions_v1.ListProjectsResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	user := ctx.Value(types.UserContextKey).(*types.User)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	req.Msg.OwnerId = &common_v1.UUID{
		Value: user.ID.String(),
	}

	projects, err := ghs.store.ListProjects(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()
	return connect.NewResponse(projects), nil
}
