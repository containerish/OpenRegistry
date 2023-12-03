package router

import (
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/config"
	github_actions_server "github.com/containerish/OpenRegistry/services/kon/github_actions/v1/server"
	"github.com/containerish/OpenRegistry/store/v1/automation"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func RegisterGitHubRoutes(
	router *echo.Group,
	cfg *config.Integration,
	env config.Environment,
	authConfig *config.Auth,
	logger telemetry.Logger,
	allowedEndpoints []string,
	usersStore vcs.VCSStore,
	automationStore automation.BuildAutomationStore,
) {
	if cfg != nil && cfg.Enabled {
		ghAppApi := github.NewGithubApp(
			cfg,
			usersStore,
			logger,
			allowedEndpoints,
			env,
		)

		ghAppApi.RegisterRoutes(router)
		githubMux := github_actions_server.NewGithubActionsServer(
			cfg,
			authConfig,
			logger,
			automationStore,
			usersStore,
		)
		go func() {
			hostPort := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
			color.Green("connect-go gRPC running on: %s", hostPort)
			if err := http.ListenAndServe(hostPort, h2c.NewHandler(githubMux, &http2.Server{})); err != nil {
				color.Red("gRPC listen error: %s", err)
			}
		}()
	}
}
