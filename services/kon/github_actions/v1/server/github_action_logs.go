package server

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"

	"connectrpc.com/connect"
	"github.com/google/go-github/v56/github"
)

func (ghs *GitHubActionsServer) getLogsToStream(
	ctx context.Context,
	githubClient *github.Client,
	logsURL string,
) ([]WorkflowStep, error) {
	downloadLogsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, logsURL, nil)
	if err != nil {
		ghs.logger.Debug().Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	downloadLogsReq.Header.Set("Accept", "application/json")
	downloadLogsResp, err := githubClient.BareDo(ctx, downloadLogsReq)
	if err != nil {
		ghs.logger.Debug().Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer downloadLogsResp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, downloadLogsResp.Body)
	if err != nil {
		ghs.logger.Debug().Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		ghs.logger.Debug().Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent := ghs.logger.Debug().Int("zip_file_count", len(zipReader.File))
	var errList []error
	var workflowSteps []WorkflowStep
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() {
			fileName, stepPosition := ghs.getFileInfo(file)
			fd, err := file.Open()
			if err != nil {
				errList = append(errList, err)
				logEvent.Str(fmt.Sprintf("error_zip_file_open_%s", fileName), err.Error())
				continue
			}

			var zipBuf bytes.Buffer
			if _, err = zipBuf.ReadFrom(fd); err != nil {
				fd.Close()
				errList = append(errList, err)
				logEvent.Str(fmt.Sprintf("error_zip_read_from_%s", fileName), err.Error())
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

	logEvent.Str("start_slice_sort", "true").Int("workflow_step_count", len(workflowSteps)).Send()
	sort.SliceStable(workflowSteps, func(i, j int) bool {
		return workflowSteps[i].StepPosition < workflowSteps[j].StepPosition
	})
	ghs.logger.Debug().Str("done_slice_sort", "true").Send()
	if len(errList) > 0 {
		return nil, fmt.Errorf("%v", errList)
	}

	return workflowSteps, nil
}
