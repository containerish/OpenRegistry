package server

import (
	"context"
	"fmt"

	connect_go "github.com/bufbuild/connect-go"
	v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/google/go-github/v50/github"
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
	// githubClient := gha.getGithubClientFromContext(ctx)
	// if err := req.Msg.Validate(); err != nil {
	// 	return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	// }
	//
	// errList := []error{}
	// for _, runID := range req.Msg.GetJobIds() {
	// 	_, err := githubClient.Actions.DeleteWorkflowRun(ctx, req.Msg.GetOwnerId(), req.Msg.GetRepo(), runID)
	// 	if err != nil {
	// 		errList = append(errList, err)
	// 	}
	// }
	//
	// if len(errList) > 0 {
	// 	return nil, connect_go.NewError(connect_go.CodeInternal, fmt.Errorf("%v", errList))
	// }
	//
	// return connect_go.NewResponse(&v1.BulkDeleteBuildJobsResponse{
	// 	Message: fmt.Sprintf("%d build jobs deleted successfully", len(req.Msg.GetJobIds())),
	// }), nil
	gha.logger.Debug().Str("procedure", req.Spec().Procedure).Bool("method_not_implemented", true).Send()
	return nil,
		connect_go.NewError(
			connect_go.CodeUnimplemented,
			fmt.Errorf("bulk job deletion is not supported by GitHub Actions integration"),
		)
}

// CancelBuild implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) CancelBuild(
	ctx context.Context,
	req *connect_go.Request[v1.CancelBuildRequest],
) (
	*connect_go.Response[v1.CancelBuildResponse],
	error,
) {
	logEvent := gha.logger.Debug().Str("procedure", req.Spec().Procedure)
	githubClient := gha.getGithubClientFromContext(ctx)
	user := ctx.Value(OpenRegistryUserContextKey).(*types.User)

	if err := req.Msg.Validate(); err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	_, err := githubClient.Actions.CancelWorkflowRunByID(
		ctx,
		user.Identities.GetGitHubIdentity().Username,
		req.Msg.GetRepo(),
		req.Msg.GetRunId(),
	)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	logEvent.Bool("success", true).Send()
	return connect_go.NewResponse(&v1.CancelBuildResponse{
		Message: "build job canceled successfully",
	}), nil
}

// DeleteJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
// We don't support deleting jobs for GitHub Action Jobs since they're included in User's GitHub Plan & keeping them
// or deleting them isn't the best we can provide to the user.
func (gha *GitHubActionsServer) DeleteJob(
	ctx context.Context,
	req *connect_go.Request[v1.DeleteJobRequest],
) (
	*connect_go.Response[v1.DeleteJobResponse],
	error,
) {
	// githubClient := gha.getGithubClientFromContext(ctx)
	// user := ctx.Value(OpenRegistryUserContextKey).(*types.User)
	// if err := req.Msg.Validate(); err != nil {
	// 	return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	// }

	// _, err := githubClient.Actions.DeleteWorkflowRun(ctx, user.Username, req.Msg.GetRepo(), req.Msg.GetRunId())
	// if err != nil {
	// 	return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	// }

	gha.logger.Debug().Str("procedure", req.Spec().Procedure).Bool("method_not_implemented", true).Send()
	return nil,
		connect_go.NewError(
			connect_go.CodeUnimplemented,
			fmt.Errorf("job deletion is not supported by GitHub Actions integration"),
		)
}

// GetBuildJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) GetBuildJob(
	ctx context.Context,
	req *connect_go.Request[v1.GetBuildJobRequest],
) (
	*connect_go.Response[v1.GetBuildJobResponse],
	error,
) {
	logEvent := gha.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	user := ctx.Value(OpenRegistryUserContextKey).(*types.User)

	githubClient := gha.getGithubClientFromContext(ctx)

	job, _, err := githubClient.Actions.GetWorkflowRunByID(
		ctx,
		user.Identities.GetGitHubIdentity().Username,
		req.Msg.GetRepo(),
		req.Msg.GetJobId(),
	)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	resp := connect_go.NewResponse(&v1.GetBuildJobResponse{
		Id:          job.GetID(),
		LogsUrl:     job.GetLogsURL(),
		Status:      job.GetStatus(),
		TriggeredBy: job.GetActor().GetLogin(),
		// Duration:    durationpb.New(job.GetRunStartedAt().Sub()),
		Branch:      job.GetHeadBranch(),
		CommitHash:  job.GetHeadCommit().GetSHA(),
		TriggeredAt: timestamppb.New(job.GetCreatedAt().UTC()),
		OwnerId:     job.GetRepository().GetOwner().GetLogin(),
	})

	logEvent.Bool("success", true).Send()
	return resp, nil
}

// ListBuildJobs implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) ListBuildJobs(
	ctx context.Context,
	req *connect_go.Request[v1.ListBuildJobsRequest],
) (
	*connect_go.Response[v1.ListBuildJobsResponse],
	error,
) {
	logEvent := gha.logger.Debug().Str("procedure", req.Spec().Procedure)
	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	user := ctx.Value(OpenRegistryUserContextKey).(*types.User)
	githubClient := gha.getGithubClientFromContext(ctx)
	ghaJobs, _, err := githubClient.Actions.ListRepositoryWorkflowRuns(
		ctx,
		user.Identities.GetGitHubIdentity().Username,
		req.Msg.GetRepo(),
		&github.ListWorkflowRunsOptions{
			// Branch:      "openregistry-build-automation",
			ListOptions: github.ListOptions{Page: 0, PerPage: 75},
		},
	)

	// jobs, err := gha.store.ListBuildJobs(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Any("request_body", req.Msg).Send()
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	jobs := []*v1.GetBuildJobResponse{}

	for _, job := range ghaJobs.WorkflowRuns {
		if job.GetID() > 0 && job.GetName() == "Build Container Image" {
			jobs = append(jobs, &v1.GetBuildJobResponse{
				Id:          job.GetID(),
				LogsUrl:     job.GetLogsURL(),
				Status:      job.GetStatus(),
				TriggeredBy: job.GetActor().GetLogin(),
				// Duration:    &durationpb.Duration{},
				Branch:      job.GetHeadBranch(),
				CommitHash:  job.GetHeadCommit().GetSHA(),
				TriggeredAt: timestamppb.New(job.GetCreatedAt().UTC()),
				OwnerId:     user.ID.String(),
			})
		}
	}

	logEvent.Bool("success", true).Send()
	return connect_go.NewResponse(&v1.ListBuildJobsResponse{Jobs: jobs}), nil
}

// StoreJob implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) StoreJob(
	ctx context.Context,
	req *connect_go.Request[v1.StoreJobRequest],
) (*connect_go.Response[v1.StoreJobResponse], error) {
	// err := req.Msg.Validate()
	// if err != nil {
	// 	return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	// }
	//
	// req.Msg.TriggeredAt = timestamppb.New(time.Now())
	// if err = gha.store.StoreJob(ctx, req.Msg); err != nil {
	// 	return nil, connect_go.NewError(connect_go.CodeInternal, err)
	// }

	// return connect_go.NewResponse(&v1.StoreJobResponse{
	// 	Message: "job stored successfully",
	// }), nil

	gha.logger.Debug().Str("procedure", req.Spec().Procedure).Bool("method_not_implemented", true).Send()
	return nil, connect_go.NewError(
		connect_go.CodeUnimplemented,
		fmt.Errorf("job storing is not supported by GitHub Actions integration"),
	)
}

// TriggerBuild implements github_actions_v1connect.GithubActionsBuildServiceHandler
func (gha *GitHubActionsServer) TriggerBuild(
	ctx context.Context,
	req *connect_go.Request[v1.TriggerBuildRequest],
) (
	*connect_go.Response[v1.TriggerBuildResponse],
	error,
) {
	logEvent := gha.logger.Debug().Str("procedure", req.Spec().Procedure)
	githubClient := gha.getGithubClientFromContext(ctx)
	user := ctx.Value(OpenRegistryUserContextKey).(*types.User)

	if err := req.Msg.Validate(); err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	_, err := githubClient.Actions.RerunWorkflowByID(
		ctx,
		user.Identities.GetGitHubIdentity().Username,
		req.Msg.GetRepo(),
		req.Msg.GetRunId(),
	)

	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	logEvent.Bool("success", true).Send()
	return connect_go.NewResponse(&v1.TriggerBuildResponse{
		Message: "job triggered successfully",
	}), nil
}
