package server

import (
	"log"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/containerish/OpenRegistry/config"
	connect_v1 "github.com/containerish/OpenRegistry/services/kone/github_actions/v1/github_actions_v1connect"
	build_automation_store "github.com/containerish/OpenRegistry/store/postgres/build_automation"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"github.com/google/go-github/v50/github"
)

type GitHubActionsServer struct {
	logger    telemetry.Logger
	config    *config.Integration
	github    *github.Client
	transport *ghinstallation.AppsTransport
	store     build_automation_store.BuildAutomationStore
}

func NewGithubActionsServer(
	config *config.Integration,
	logger telemetry.Logger,
	mux *http.ServeMux,
	store build_automation_store.BuildAutomationStore,
) {
	transport, githubClient, err := newGHClient(config.AppID, config.PrivateKeyPem)
	if err != nil {
		log.Fatalf("%s\n", color.RedString("error creating github client: %s", err))
	}

	server := &GitHubActionsServer{
		logger:    logger,
		config:    config,
		transport: transport,
		github:    githubClient,
		store:     store,
	}

	logsPath, logsHandler := connect_v1.NewGitHubActionsLogsServiceHandler(server)
	projectsPath, projectsHandler := connect_v1.NewGitHubActionsProjectServiceHandler(server)
	mux.Handle(logsPath, logsHandler)
	mux.Handle(projectsPath, projectsHandler)
}
