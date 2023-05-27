package server

import (
	"net/url"

	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	connect_go "github.com/bufbuild/connect-go"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	github_impl "github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
	"github.com/google/go-github/v50/github"
)

func (ghs *GitHubActionsServer) StreamWorkflowJobLogs(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.StreamWorkflowJobLogsRequest],
	stream *connect_go.ServerStream[github_actions_v1.StreamWorkflowJobLogsResponse],
) error {
	if err := req.Msg.Validate(); err != nil {
		return connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	githubAppInstallation, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		ghs.logger.Debug().Str("ListRepositoryWorkflowRuns", "missing githubAppInstallation").Send()
		return connect_go.NewError(connect_go.CodeInternal, fmt.Errorf("missing githubAppInstallation"))
	}

	uri, err := ghs.getWorkflowJobLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), req.Msg.GetJobId(), githubAppInstallation)
	if err != nil {
		return connect_go.NewError(connect_go.CodeNotFound, err)
	}

	downloadLogsReq, err := http.NewRequest(http.MethodGet, uri.String(), nil)
	if err != nil {
		return connect_go.NewError(connect_go.CodeInternal, err)
	}

	downloadLogsResp, err := ghs.github.BareDo(ctx, downloadLogsReq)
	if err != nil {
		return connect_go.NewError(connect_go.CodeInternal, err)
	}
	defer downloadLogsResp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, downloadLogsResp.Body)
	if err != nil {
		return connect_go.NewError(connect_go.CodeInternal, err)
	}

	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		return connect_go.NewError(connect_go.CodeInternal, err)
	}

	var errList []error
	var workflowSteps []WorkflowStep
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() && strings.HasPrefix(file.Name, "Build/") {
			nameParts := strings.Split(file.Name, "_")
			fileName := strings.TrimSuffix(nameParts[1], ".txt")
			stepPosition, _ := strconv.ParseInt(strings.Split(nameParts[0], "/")[1], 10, 64)

			fd, err := file.Open()
			if err != nil {
				errList = append(errList, err)
				continue
			}

			var zipBuf bytes.Buffer
			if _, err = zipBuf.ReadFrom(fd); err != nil {
				fd.Close()
				errList = append(errList, err)
				continue
			}
			fd.Close()

			workflowSteps = append(workflowSteps, WorkflowStep{
				StepPosition: stepPosition,
				Title:        fileName,
				Buf:          zipBuf,
			})
		}
	}

	sort.SliceStable(workflowSteps, func(i, j int) bool {
		return workflowSteps[i].StepPosition < workflowSteps[j].StepPosition
	})

	for _, step := range workflowSteps {
		stream.Send(&github_actions_v1.StreamWorkflowJobLogsResponse{
			LogMessage: step.Buf.String(),
		})

	}
	if len(errList) > 0 {
		stream.Send(&github_actions_v1.StreamWorkflowJobLogsResponse{
			LogMessage: fmt.Sprintf("%v", errList),
		})
	}
	stream.Send(&github_actions_v1.StreamWorkflowJobLogsResponse{LogMessage: "STREAM_END"})
	return nil
}

type WorkflowStep struct {
	Buf          bytes.Buffer
	Title        string
	StepPosition int64
}

func (ghs *GitHubActionsServer) streamPreviousRunLogs(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect_go.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
	githubClient *github.Client,
) error {
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
		return connect_go.NewError(connect_go.CodeUnavailable, err)
	}

	if len(runs.WorkflowRuns) < 1 {
		return connect_go.NewError(connect_go.CodeInvalidArgument, fmt.Errorf("no github actions run found for this id"))
	}
	logEvent := ghs.logger.Debug().Str("method", "streamPreviousRunLogs")

	workflowSteps, err := ghs.getLogsToStream(ctx, githubClient, runs.WorkflowRuns[0].GetLogsURL())
	if err != nil {
		logEvent.Err(err).Send()
		return err
	}

	for _, step := range workflowSteps {
		stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: step.Buf.String(),
		})
	}

	return stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{LogMessage: "STREAM_END"})
}

func (ghs *GitHubActionsServer) StreamWorkflowRunLogs(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect_go.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
) error {
	// color.Red("run id: %d - repo name: %s - repo owner: %s", req.Msg.GetRunId(), req.Msg.GetRepoName(), req.Msg.GetRepoOwner())
	ghs.logger.Debug().
		Str("method", "StreamWorkflowRunLogs").
		Str("workflow_run_id", fmt.Sprintf("%d", req.Msg.GetRunId())).
		Str("repo_name", req.Msg.GetRepoName()).
		Str("repo_owner", req.Msg.GetRepoOwner()).
		Bool("skip_to_previous_run", req.Msg.SkipToPreviousRun).
		Send()
	if err := req.Msg.Validate(); err != nil {
		return connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	logEvent := ghs.logger.Debug().Str("msg_validation", "success")

	// get the GitHub App Installation ID, which must be set by the interceptor
	ghAppInstallationID, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		return connect_go.NewError(connect_go.CodeInvalidArgument, fmt.Errorf("github app installation id not found for user"))
	}

	githubClient := ghs.refreshGHClient(ghs.transport, ghAppInstallationID)
	if req.Msg.GetSkipToPreviousRun() {
		logEvent.Bool("skip_to_previous_run", true).Send()
		return ghs.streamPreviousRunLogs(ctx, req, stream, githubClient)
	}

	ghs.activeLogStreamJobs[ghs.getLogsEventKey(req.Msg)] = &streamLogsJob{req: req.Msg}
	logEvent.Bool("waiting_for_log_events", true).Send()

	jobEndTime := time.Now().Add(time.Minute * 30).Unix()
	found := false

	for jobEndTime >= time.Now().Unix() {
		ghs.logger.Debug().
			Str("method", "StreamWorkflowRunLogs").
			Str("step", "waiting_for_logs").
			Send()
		ghs.mu.Lock()
		event, event_found := ghs.activeLogStreamJobs[ghs.getLogsEventKey(req.Msg)]
		ghs.mu.Unlock()
		if !event_found {
			continue
		}
		if event.req.GetRunId() > 0 && event.action == "completed" {
			// color.Green("found id for workflow run event")
			ghs.logger.Debug().
				Str("method", "StreamWorkflowRunLogs").
				Str("step", "workflow_if_found").
				Str("workflow_run_id", fmt.Sprintf("%d", event.req.GetRunId())).
				Send()
			found = true
			stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
				LogMessage: "Fetching logs...",
			})
			break
		}
		stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: "Waiting for logs...",
		})
		// wait before trying next run
		time.Sleep(time.Second * 2)
	}

	if !found {
		return connect_go.NewError(connect_go.CodeInternal, fmt.Errorf("error getting github logs event run id"))
	}

	delete(ghs.activeLogStreamJobs, ghs.getLogsEventKey(req.Msg))
	stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
		LogMessage: "Downloading logs to stream",
	})

	githubAppInstallation, ok := ctx.Value(github_impl.GithubInstallationIDContextKey).(int64)
	if !ok {
		ghs.logger.Debug().Str("ListRepositoryWorkflowRuns", "missing githubAppInstallation").Send()
		return connect_go.NewError(connect_go.CodeInternal, fmt.Errorf("missing githubAppInstallation"))
	}
	client := ghs.refreshGHClient(ghs.transport, githubAppInstallation)

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
		ghs.logger.Debug().Str("ListRepositoryWorkflowRuns", err.Error()).Send()
		return connect_go.NewError(connect_go.CodeInternal, err)
	}

	runID := actionRuns.WorkflowRuns[0].GetID()
	uri, err := ghs.getWorkflowRunLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), runID)
	if err != nil {
		ghs.logger.Debug().
			Str("method", "StreamWorkflowRunLogs").
			Str("step", "getWorkflowRunLogsURL").
			Str("error", err.Error()).
			Send()
		// color.Red("error getting download url for logs: %s", err)
		return connect_go.NewError(connect_go.CodeNotFound, err)
	}

	workflowSteps, err := ghs.getLogsToStream(ctx, githubClient, uri.String())
	if err != nil {
		ghs.logger.Debug().Err(err).Send()
		return err
	}

	for _, step := range workflowSteps {
		stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: step.Buf.String(),
		})

	}
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

func (ghs *GitHubActionsServer) refreshGHClient(appTransport *ghinstallation.AppsTransport, id int64) *github.Client {
	transport := ghinstallation.NewFromAppsTransport(appTransport, id)
	return github.NewClient(&http.Client{Transport: transport})
}

func (ghs *GitHubActionsServer) getWorkflowJobLogsURL(ctx context.Context, owner, repo string, jobID, githubAppInstallationID int64) (*url.URL, error) {
	client := ghs.refreshGHClient(ghs.transport, githubAppInstallationID)
	url, resp, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, true)
	if err != nil {
		var buf bytes.Buffer
		_, bufReadErr := buf.ReadFrom(resp.Body)
		// resp.Body.Close()
		if bufReadErr != nil {
			return nil, fmt.Errorf("error reading response from github: %w", bufReadErr)
		}

		return nil, fmt.Errorf("error getting the url: %s", buf.String())
	}

	return url, nil
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

	logEvent := ghs.logger.Debug().
		Str("method", "retryGetWorkflowRunLogsURL").
		Str("owner", owner).
		Str("repo", repo).
		Int64("runId", runID)

	for i := 0; i < retryCount; i++ {
		logEvent.Int("retry_count", i)
		time.Sleep(backoff)
		url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, true)
		if err != nil {
			var buf bytes.Buffer
			_, bufReadErr := buf.ReadFrom(resp.Body)
			// resp.Body.Close()
			if bufReadErr != nil {
				logEvent.Str("err_buf_read", bufReadErr.Error())
				logEvent.Send()
				continue
			}

			logsErr = fmt.Errorf("error getting the url: %s", buf.String())
			logEvent.Str("err_read_url", buf.String())
			logEvent.Send()
			continue
		}

		logEvent.Send()
		logsUrl = url
		logsErr = nil
		break
	}

	return logsUrl, logsErr

}

func (ghs *GitHubActionsServer) getWorkflowRunLogsURL(ctx context.Context, owner, repo string, runID int64) (*url.URL, error) {
	client := ghs.refreshGHClient(ghs.transport, 30257283)

	return ghs.retryGetWorkflowRunLogsURL(client, ctx, owner, repo, runID, 3, time.Second*5)
	// url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, true)
	// if err != nil {
	// 	var buf bytes.Buffer
	// 	_, bufReadErr := buf.ReadFrom(resp.Body)
	// 	// resp.Body.Close()
	// 	if bufReadErr != nil {
	// 		return nil, fmt.Errorf("error reading response from github: %w", bufReadErr)
	// 	}
	//
	// 	return nil, fmt.Errorf("error getting the url: %s", buf.String())
	// }
	//
	// return url, nil
}

func (ghs *GitHubActionsServer) DumpLogs(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.DumpLogsRequest],
) (
	*connect_go.Response[github_actions_v1.DumpLogsResponse],
	error,
) {
	color.Red("run id: %d - repo name: %s - repo owner: %s", req.Msg.GetRunId(), req.Msg.GetRepoName(), req.Msg.GetRepoOwner())
	if err := req.Msg.Validate(); err != nil {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	uri, err := ghs.getWorkflowRunLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), req.Msg.GetRunId())
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeNotFound, err)
	}

	downloadLogsReq, err := http.NewRequest(http.MethodGet, uri.String(), nil)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	downloadLogsReq.Header.Set("Accept", "application/zip")

	downloadLogsResp, err := ghs.github.BareDo(ctx, downloadLogsReq)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}
	defer downloadLogsResp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, downloadLogsResp.Body)
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		return nil, connect_go.NewError(connect_go.CodeInternal, err)
	}

	var errList []error
	var workflowSteps []WorkflowStep
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() && strings.HasPrefix(file.Name, "Build/") {
			nameParts := strings.Split(file.Name, "_")
			fileName := strings.TrimSuffix(nameParts[1], ".txt")
			stepPosition, _ := strconv.ParseInt(strings.Split(nameParts[0], "/")[1], 10, 64)

			fd, err := file.Open()
			if err != nil {
				errList = append(errList, err)
				continue
			}

			var zipBuf bytes.Buffer
			if _, err = zipBuf.ReadFrom(fd); err != nil {
				fd.Close()
				errList = append(errList, err)
				continue
			}
			fd.Close()

			workflowSteps = append(workflowSteps, WorkflowStep{
				StepPosition: stepPosition,
				Title:        fileName,
				Buf:          zipBuf,
			})
		}
	}

	sort.SliceStable(workflowSteps, func(i, j int) bool {
		return workflowSteps[i].StepPosition < workflowSteps[j].StepPosition
	})

	var logs []string
	for _, step := range workflowSteps {
		// stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
		// 	LogMessage: step.Buf.String(),
		// })

		logs = append(logs, step.Buf.String())

	}
	if len(errList) > 0 {
		return nil, connect_go.NewError(connect_go.CodeInvalidArgument, fmt.Errorf("%v", errList))
	}

	return connect_go.NewResponse(&github_actions_v1.DumpLogsResponse{
		Logs: logs,
	}), nil
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
