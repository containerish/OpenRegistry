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
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kone/github_actions/v1"
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

	uri, err := ghs.getWorkflowJobLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), req.Msg.GetJobId())
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

func (ghs *GitHubActionsServer) StreamWorkflowRunLogs(
	ctx context.Context,
	req *connect_go.Request[github_actions_v1.StreamWorkflowRunLogsRequest],
	stream *connect_go.ServerStream[github_actions_v1.StreamWorkflowRunLogsResponse],
) error {
	color.Red("run id: %d - repo name: %s - repo owner: %s", req.Msg.GetRunId(), req.Msg.GetRepoName(), req.Msg.GetRepoOwner())
	if err := req.Msg.Validate(); err != nil {
		return connect_go.NewError(connect_go.CodeInvalidArgument, err)
	}

	uri, err := ghs.getWorkflowRunLogsURL(ctx, req.Msg.GetRepoOwner(), req.Msg.GetRepoName(), req.Msg.GetRunId())
	if err != nil {
		return connect_go.NewError(connect_go.CodeNotFound, err)
	}

	downloadLogsReq, err := http.NewRequest(http.MethodGet, uri.String(), nil)
	if err != nil {
		return connect_go.NewError(connect_go.CodeInternal, err)
	}

	downloadLogsReq.Header.Set("Accept", "application/zip")

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
		stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: step.Buf.String(),
		})

	}
	if len(errList) > 0 {
		stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
			LogMessage: fmt.Sprintf("%v", errList),
		})
	}
	stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{LogMessage: "STREAM_END"})
	return nil
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

func (ghs *GitHubActionsServer) getWorkflowJobLogsURL(ctx context.Context, owner, repo string, jobID int64) (*url.URL, error) {
	client := ghs.refreshGHClient(ghs.transport, 30257283)
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

func (ghs *GitHubActionsServer) getWorkflowRunLogsURL(ctx context.Context, owner, repo string, runID int64) (*url.URL, error) {
	client := ghs.refreshGHClient(ghs.transport, 30257283)
	url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, true)
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
		// stream.Send(&github_actions_v1.StreamWorkflowRunLogsResponse{
		// 	LogMessage: fmt.Sprintf("%v", errList),
		// })
	}

	return connect_go.NewResponse(&github_actions_v1.DumpLogsResponse{
		Logs: logs,
	}), nil
}
