package server

import (
	"context"
	"time"

	connect_go "github.com/bufbuild/connect-go"
	v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// BulkDeleteBuildJobs implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) BulkDeleteBuildJobs(
	ctx context.Context,
	req *connect_go.Request[v1.BulkDeleteBuildJobsRequest],
) (
	*connect_go.Response[v1.BulkDeleteBuildJobsResponse],
	error,
) {
	panic("unimplemented")
}

// CancelBuild implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) CancelBuild(
	ctx context.Context,
	req *connect_go.Request[v1.CancelBuildRequest],
) (
	*connect_go.Response[v1.CancelBuildResponse],
	error,
) {
	panic("unimplemented")
}

// DeleteJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) DeleteJob(
	ctx context.Context,
	req *connect_go.Request[v1.DeleteJobRequest],
) (
	*connect_go.Response[v1.DeleteJobResponse],
	error,
) {
	panic("unimplemented")
}

// GetBuildJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) GetBuildJob(
	ctx context.Context,
	req *connect_go.Request[v1.GetBuildJobRequest],
) (
	*connect_go.Response[v1.GetBuildJobResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	job, err := gha.store.GetBuildJob(ctx, req.Msg)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	return connect_go.NewResponse(job), nil
}

// ListBuildJobs implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) ListBuildJobs(
	ctx context.Context,
	req *connect_go.Request[v1.ListBuildJobsRequest],
) (
	*connect_go.Response[v1.ListBuildJobsResponse],
	error,
) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	jobs, err := gha.store.ListBuildJobs(ctx, req.Msg)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	return connect_go.NewResponse(jobs), nil
}

// StoreJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) StoreJob(ctx context.Context, req *connect_go.Request[v1.StoreJobRequest]) (*connect_go.Response[v1.StoreJobResponse], error) {
	err := req.Msg.Validate()
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	req.Msg.TriggeredAt = timestamppb.New(time.Now())
	if err = gha.store.StoreJob(ctx, req.Msg); err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	return connect_go.NewResponse(&v1.StoreJobResponse{
		Message: "job stored successfully",
	}), nil
}

// TriggerBuild implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) TriggerBuild(
	ctx context.Context,
	req *connect_go.Request[v1.TriggerBuildRequest],
) (
	*connect_go.Response[v1.TriggerBuildResponse],
	error,
) {
	panic("unimplemented")
}
