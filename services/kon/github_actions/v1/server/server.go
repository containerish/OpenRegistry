package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/bufbuild/connect-go"
	"github.com/containerish/OpenRegistry/config"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	connect_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1/github_actions_v1connect"
	build_automation_store "github.com/containerish/OpenRegistry/store/postgres/build_automation"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/fatih/color"
	"github.com/google/go-github/v50/github"
)

type GitHubActionsServer struct {
	logger              telemetry.Logger
	config              *config.Integration
	github              *github.Client
	transport           *ghinstallation.AppsTransport
	store               build_automation_store.BuildAutomationStore
	activeLogStreamJobs map[string]*streamLogsJob
	mu                  *sync.RWMutex
}

type streamLogsJob struct {
	req    *github_actions_v1.StreamWorkflowRunLogsRequest
	action string
}

func NewGithubActionsServer(
	config *config.Integration,
	logger telemetry.Logger,
	mux *http.ServeMux,
	store build_automation_store.BuildAutomationStore,
	ghStore vcs.VCSStore,
) {
	transport, githubClient, err := newGHClient(config.AppID, config.PrivateKeyPem)
	if err != nil {
		log.Fatalf("%s\n", color.RedString("error creating github client: %s", err))
	}

	server := &GitHubActionsServer{
		logger:              logger,
		config:              config,
		transport:           transport,
		github:              githubClient,
		store:               store,
		activeLogStreamJobs: make(map[string]*streamLogsJob),
		mu:                  &sync.RWMutex{},
	}

	interceptors := connect.WithInterceptors(NewGithubAppInterceptor(logger, ghStore, nil))

	mux.Handle(connect_v1.NewGitHubActionsLogsServiceHandler(server, interceptors))
	mux.Handle(connect_v1.NewGitHubActionsProjectServiceHandler(server))
	mux.Handle(connect_v1.NewGithubActionsBuildServiceHandler(server))
	// mux.Handle("/", http.HandlerFunc(server.Listen)
	mux.Handle("/services.kon.github_actions.v1.GitHubWebhookListenerService/Listen", http.HandlerFunc(server.Listen))
	// mux.Handle(, mux.Handle(pattern string, handler http.Handler)
}
