package server

import (
	"fmt"
	"net/http"

	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/fatih/color"
	"github.com/google/go-github/v56/github"
)

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
		ghs.logger.Debug().
			Str("method", "Listen").
			Str("error", err.Error()).
			Send()
		fmt.Fprintf(resp, "%s", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(req), payload)
	if err != nil {
		ghs.logger.Debug().
			Str("method", "Listen").
			Str("error", err.Error()).
			Send()

		fmt.Fprintf(resp, "%s", err)
		return
	}

	switch event := event.(type) {
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
		ghs.logger.Debug().
			Str("method", "Listen").
			Str("github_event", "WorkflowRunEvent").
			Int64("workflow_id", event.GetWorkflowRun().GetID()).
			Str("event_key", eventKey).
			Send()
		job, ok := ghs.activeLogStreamJobs[eventKey]
		if ok && job.req.GetRunId() == 0 {
			ghs.activeLogStreamJobs[eventKey] = val
		}

		if ok && job.req.GetRunId() > 0 {
			if event.GetAction() == "completed" {
				color.Green("action is competed")
				ghs.activeLogStreamJobs[eventKey] = val
			}
		}
	}
}
