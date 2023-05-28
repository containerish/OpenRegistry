package server

import (
	"fmt"
	"net/http"

	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/fatih/color"
	"github.com/google/go-github/v50/github"
	// anypb "google.golang.org/protobuf/types/known/anypb"
	// "github.com/google/go-github/v50/github"
)

//nolint:cyclop
func (ghs *GitHubActionsServer) Listen(resp http.ResponseWriter, req *http.Request) {
	xHubSignature := req.Header.Get("X-Hub-Signature-256")
	contentType := req.Header.Get("Content-Type")

	payload, err := github.ValidatePayloadFromBody(
		contentType,
		req.Body,
		xHubSignature,
		[]byte(ghs.config.WebhookSecret),
	)

	if err != nil {
		ghs.logger.Log(nil, nil).
			Str("method", "Listen").
			Str("error", err.Error()).
			Send()
		// color.Yellow("err in ValidatePayloadFromBody: %s", err)
		// echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
		// 	"error": err.Error(),
		// })
		// gh.logger.Log(ctx, err).Send()
		fmt.Fprintf(resp, "%s", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		ghs.logger.Log(nil, nil).
			Str("method", "Listen").
			Str("error", err.Error()).
			Send()

		// color.Yellow("err ParseWebHook: %s", err)
		// echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
		// 	"error": err.Error(),
		// })
		// gh.logger.Log(ctx, err).Send()
		fmt.Fprintf(resp, "%s", err)
		return
	}

	switch event := event.(type) {
	case *github.PingEvent:
		ghs.logger.Log(nil, nil).
			Str("method", "Listen").
			Str("github_event", "PingEvent").
			Str("sender", event.GetSender().GetName()).
			Send()
	case *github.WorkflowJobEvent:
		ghs.logger.Log(nil, nil).
			Str("method", "Listen").
			Str("github_event", "WorkflowJobEvent").
			Str("status", event.GetAction()).
			Str("workflow_name", event.GetWorkflowJob().GetName()).
			Str("workflow_id", fmt.Sprintf("%d", event.GetWorkflowJob().GetID())).
			Str("workflow_run_id", fmt.Sprintf("%d", event.GetWorkflowJob().GetRunID())).
			Send()
	case *github.WorkflowRunEvent:
		val := &streamLogsJob{
			req: &github_actions_v1.StreamWorkflowRunLogsRequest{
				RepoOwner: event.GetRepo().GetOwner().GetLogin(),
				RepoName:  event.GetRepo().GetName(),
				RunId:     event.GetWorkflowRun().GetID(),
			},
			action: event.GetAction(),
		}
		eventKey := ghs.getLogsEventKey(val.req)
		ghs.logger.Log(nil, nil).
			Str("method", "Listen").
			Str("github_event", "WorkflowRunEvent").
			Str("status", event.GetAction()).
			Str("workflow_name", event.GetWorkflowRun().GetName()).
			Str("workflow_id", fmt.Sprintf("%d", event.GetWorkflowRun().GetID())).
			Str("event_key", eventKey).
			Send()
		job, ok := ghs.activeLogStreamJobs[eventKey]
		if ok && job.req.GetRunId() == 0 {
			// color.Green("setting the job information")
			ghs.activeLogStreamJobs[eventKey] = val
		}

		if ok && job.req.GetRunId() > 0 {
			if event.GetAction() == "completed" {
				color.Green("action is competed")
				ghs.activeLogStreamJobs[eventKey] = val
			}
		}

	case *github.WorkflowDispatchEvent:
		ghs.logger.Log(nil, nil).
			Str("github_event", "WorkflowDispatchEvent").
			Str("workflow_name", event.GetWorkflow()).
			Send()
	case *github.InstallationRepositoriesEvent:
	case *github.CheckRunEvent:
	case *github.InstallationEvent:
	case *github.CheckSuiteEvent:
		ghs.logger.Log(nil, nil).
			Str("github_event", "CheckSuiteEvent").
			Str("status", event.GetAction()).
			Str("check_suite_status", event.CheckSuite.GetStatus()).
			Str("check_suite_id", fmt.Sprintf("%d", event.GetCheckSuite().GetID())).
			Send()
	default:
		ghs.logger.Log(nil, nil).
			Str("github_event", "Default--Nothing-Matched").
			Any("event_unknown", fmt.Sprintf("%T", event)).
			Send()
		// Str("workflow_name", event)
		// color.Yellow("event type default: %#v", event)
	}
}
