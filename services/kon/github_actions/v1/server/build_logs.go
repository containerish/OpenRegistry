package server

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v56/github"

	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	github_impl "github.com/containerish/OpenRegistry/vcs/github"
)

type WorkflowStep struct {
	Title        string
	Buf          bytes.Buffer
	StepPosition int64
}

func (ghs *GitHubActionsServer) streamPreviousRunLogs(
	ctx context.Context,
	req *connect.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
	githubClient *github.Client,
) error {
	logEvent := ghs.logger.Debug().Str("method", "streamPreviousRunLogs")
	runs, _, err := githubClient.Actions.ListRepositoryWorkflowRuns(
		ctx,
		req.Msg.GetRepoOwner(),
		req.Msg.GetRepoName(),
		&github.ListWorkflowRunsOptions{
			ListOptions: github.ListOptions{
				PerPage: 1,
			},
		},
	)
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeUnavailable, err)
	}

	if len(runs.WorkflowRuns) < 1 {
		errMsg := fmt.Errorf("no github actions run found for this id")
		logEvent.Err(errMsg).Send()
		return connect.NewError(connect.CodeInvalidArgument, errMsg)
	}

	workflowSteps, err := ghs.getLogsToStream(ctx, githubClient, runs.WorkflowRuns[0].GetLogsURL())
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	for _, step := range workflowSteps {
		_ = stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: step.Buf.String(),
		})
	}

	logEvent.Bool("success", true).Send()
	return stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{LogMessage: "STREAM_END"})
}

func (ghs *GitHubActionsServer) StreamWorkflowRunLogs(
	ctx context.Context,
	req *connect.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
) error {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	if err := req.Msg.Validate(); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	logEvent.
		Int64("workflow_run_id", req.Msg.GetRunId()).
		Str("repo_owner", req.Msg.GetRepoOwner()).
		Str("repo_name", req.Msg.GetRepoName())

	// get the GitHub App Installation ID, which must be set by the interceptor
	ghAppInstallationID, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		errMsg := fmt.Errorf("github app installation id not found for user")
		logEvent.Err(errMsg).Send()
		return connect.NewError(
			connect.CodeInvalidArgument, errMsg,
		)
	}

	githubClient := ghs.refreshGHClient(ghAppInstallationID)
	if req.Msg.GetSkipToPreviousRun() {
		logEvent.Bool("skip_to_previous_run", true).Send()
		return ghs.streamPreviousRunLogs(ctx, req, stream, githubClient)
	}

	ghs.activeLogStreamJobs[ghs.getLogsEventKey(req.Msg)] = &streamLogsJob{req: req.Msg}
	logEvent.Bool("waiting_for_log_events", true)

	err := ghs.waitForJobToFinish(ctx, githubClient, req, stream)
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	delete(ghs.activeLogStreamJobs, ghs.getLogsEventKey(req.Msg))
	_ = stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
		LogMessage: "Downloading logs to stream",
		MsgType:    github_actions_v1.StreamWorkflowRunMessageType_STREAM_WORKFLOW_RUN_MESSAGE_TYPE_PROCESSING,
	})

	githubAppInstallation, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		errMsg := fmt.Errorf("missing GitHub App installation ID")
		logEvent.Err(errMsg).Send()
		return connect.NewError(connect.CodeInternal, errMsg)
	}
	client := ghs.refreshGHClient(githubAppInstallation)

	actionRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(
		ctx,
		req.Msg.GetRepoOwner(),
		req.Msg.GetRepoName(),
		&github.ListWorkflowRunsOptions{
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 1,
			},
		},
	)
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeInternal, err)
	}

	runID := actionRuns.WorkflowRuns[0].GetID()
	uri, err := ghs.getWorkflowRunLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), runID)
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeNotFound, err)
	}

	workflowSteps, err := ghs.getLogsToStream(ctx, githubClient, uri.String())
	if err != nil {
		logEvent.Err(err).Send()
		return err
	}

	for _, step := range workflowSteps {
		_ = stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: step.Buf.String(),
			MsgType:    github_actions_v1.StreamWorkflowRunMessageType_STREAM_WORKFLOW_RUN_MESSAGE_TYPE_STEP,
		})
	}

	logEvent.Bool("success", true).Send()
	return stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{LogMessage: "STREAM_END"})
}

func newGHClient(appID int64, privKeyPem string) (*ghinstallation.AppsTransport, *github.Client, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, privKeyPem)
	if err != nil {
		return nil, nil, fmt.Errorf("ERR_CREATE_NEW_TRANSPORT: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: transport, Timeout: time.Second * 30})
	return transport, client, nil
}

func (ghs *GitHubActionsServer) refreshGHClient(id int64) *github.Client {
	transport := ghinstallation.NewFromAppsTransport(ghs.transport, id)
	return github.NewClient(&http.Client{Transport: transport})
}

// getGithubClientFromContext returns github client refreshed with user's github app installation id.
// This method panics if the github app installation id is not present inside the context
func (ghs *GitHubActionsServer) getGithubClientFromContext(ctx context.Context) *github.Client {
	githubAppInstallationID := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	return ghs.refreshGHClient(githubAppInstallationID)
}

func (ghs *GitHubActionsServer) retryGetWorkflowRunLogsURL(
	client *github.Client,
	ctx context.Context,
	owner string,
	repo string,
	runID int64,
	retryCount int,
	backoff time.Duration,
) (*url.URL, error) {
	var logsUrl *url.URL
	var logsErr error

	for i := 0; i < retryCount; i++ {
		time.Sleep(backoff)
		url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, github_impl.MaxGitHubRedirects)
		if err != nil {
			var buf bytes.Buffer
			_, bufReadErr := buf.ReadFrom(resp.Body)
			resp.Body.Close()
			if bufReadErr != nil {
				continue
			}

			logsErr = fmt.Errorf("error getting the url: %s", buf.String())
			continue
		}

		logsUrl = url
		logsErr = nil
		break
	}

	return logsUrl, logsErr
}

func (ghs *GitHubActionsServer) getWorkflowRunLogsURL(
	ctx context.Context,
	owner string,
	repo string,
	runID int64,
) (*url.URL, error) {
	client := ghs.refreshGHClient(30257283)

	return ghs.retryGetWorkflowRunLogsURL(client, ctx, owner, repo, runID, 3, time.Second*5)
}

func (ghs *GitHubActionsServer) DumpLogs(
	ctx context.Context,
	req *connect.Request[github_actions_v1.DumpLogsRequest],
) (
	*connect.Response[github_actions_v1.DumpLogsResponse],
	error,
) {
	logEvent := ghs.logger.Debug().Str("procedure", req.Spec().Procedure)
	if err := req.Msg.Validate(); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ghAppInstallationID, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		errMsg := fmt.Errorf("github app installation id not found for user")
		logEvent.Err(errMsg).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, errMsg)
	}

	githubClient := ghs.refreshGHClient(ghAppInstallationID)

	runs, _, err := githubClient.Actions.ListRepositoryWorkflowRuns(
		ctx,
		req.Msg.GetRepoOwner(),
		req.Msg.GetRepoName(),
		&github.ListWorkflowRunsOptions{
			ListOptions: github.ListOptions{
				PerPage: 1,
			},
		},
	)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	if len(runs.WorkflowRuns) < 1 {
		errMsg := fmt.Errorf("no github actions run found for this id")
		logEvent.Err(errMsg).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, errMsg)
	}

	workflowSteps, err := ghs.getLogsToStream(ctx, githubClient, runs.WorkflowRuns[0].GetLogsURL())
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var logs []string
	for _, step := range workflowSteps {
		logs = append(logs, step.Buf.String())
	}

	logEvent.Bool("success", true).Send()
	return connect.NewResponse(&github_actions_v1.DumpLogsResponse{Logs: logs}), nil
}

func (ghs *GitHubActionsServer) getLogsEventKey(job *github_actions_v1.StreamWorkflowRunLogsRequest) string {
	return job.GetRepoOwner() + "/" + job.GetRepoName()
}

func (ghs *GitHubActionsServer) getFileInfo(fi *zip.File) (string, int64) {
	fileName := fi.FileInfo().Name()
	stepPosition := int64(0)
	if strings.HasSuffix(fileName, ".txt") {
		nameParts := strings.Split(fileName, "_")
		stepPosition, _ = strconv.ParseInt(nameParts[0], 10, 64)
		fileName = nameParts[1]

		return fileName, stepPosition
	}

	return fileName, 0
}

func (ghs *GitHubActionsServer) waitForJobToFinish(
	ctx context.Context,
	githubClient *github.Client,
	req *connect.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
) error {
	now := time.Now()
	logEvent := ghs.logger.Debug().
		Str("method", "waitForJobToFinish").
		Int64("run_id", req.Msg.GetRunId()).
		Str("repo_name", req.Msg.GetRepoName()).
		Str("repo_owner", req.Msg.GetRepoOwner())

	workflowRun, _, err := githubClient.Actions.GetWorkflowRunByID(
		ctx,
		req.Msg.GetRepoOwner(),
		req.Msg.GetRepoName(),
		req.Msg.GetRunId(),
	)
	if err != nil {
		logEvent.Err(err).Send()
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	logEvent.Str("status", workflowRun.GetStatus())

	jobEndTime := time.Now().Add(time.Minute * 10).Unix()
	status := workflowRun.GetStatus()
	jobInProgress := status == "queued" ||
		status == "in_progress" ||
		status == "requested" ||
		status == "waiting" ||
		status == "pending"

	if !jobInProgress {
		logEvent.Str("redirect_to", "streamPreviousRunLogs").Send()
		return ghs.streamPreviousRunLogs(ctx, req, stream, githubClient)
	}

	for jobEndTime >= time.Now().Unix() && jobInProgress {
		logEvent.Bool("waiting_for_logs", true).Dur("waiting_since", time.Since(now))
		ghs.mu.Lock()
		event, event_found := ghs.activeLogStreamJobs[ghs.getLogsEventKey(req.Msg)]
		ghs.mu.Unlock()
		if !event_found {
			time.Sleep(time.Second * 2)
			logEvent.Send()
			continue
		}
		if event.req.GetRunId() > 0 && event.action == "completed" {
			logEvent.Bool("workflow_id_found", true).Send()
			_ = stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
				LogMessage: "Fetching logs...",
				MsgType:    github_actions_v1.StreamWorkflowRunMessageType_STREAM_WORKFLOW_RUN_MESSAGE_TYPE_PROCESSING,
			})
			return nil
		}
		_ = stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: "Waiting for logs...",
			MsgType:    github_actions_v1.StreamWorkflowRunMessageType_STREAM_WORKFLOW_RUN_MESSAGE_TYPE_WAIT,
		})
		// wait before trying next run
		time.Sleep(time.Second * 2)
	}

	return fmt.Errorf("job not found")
}
