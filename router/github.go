package router

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/containerish/OpenRegistry/config"
	github_actions_server "github.com/containerish/OpenRegistry/services/kon/github_actions/v1/server"
	"github.com/containerish/OpenRegistry/store/v1/automation"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/containerish/OpenRegistry/vcs/github"
)

func RegisterGitHubRoutes(
	router *echo.Group,
	cfg *config.GithubIntegration,
	env config.Environment,
	authConfig *config.Auth,
	logger telemetry.Logger,
	allowedEndpoints []string,
	usersStore vcs.VCSStore,
	automationStore automation.BuildAutomationStore,
	allowedOrigins []string,
	registryEndpoint string,
) {
	if cfg != nil && cfg.Enabled {
		ghAppApi := github.NewGithubApp(
			cfg,
			usersStore,
			logger,
			allowedEndpoints,
			env,
			registryEndpoint,
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
			addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
			color.Green("connectrpc GitHub Automation gRPC service running on: %s", addr)
			ghCors := cors.New(cors.Options{
				AllowedOrigins: allowedOrigins,
				AllowOriginFunc: func(origin string) bool {
					return strings.HasSuffix(origin, "openregistry.dev") ||
						strings.HasSuffix(origin, "cntr.sh") ||
						strings.HasSuffix(origin, "openregistry-web.pages.dev") ||
						strings.Contains(origin, "localhost")
				},
				AllowedMethods: []string{
					http.MethodOptions, http.MethodGet, http.MethodPost,
				},
				AllowedHeaders: []string{
					"Origin",
					"Content-Type",
					"Authorization",
					"Connect-Protocol-Version",
					"Connect-Timeout-Ms",
					"Grpc-Timeout",
					"X-Grpc-Web",
					"X-User-Agent",
				},
				AllowCredentials: true,
				Debug:            true,
			})
			handler := ghCors.Handler(h2c.NewHandler(githubMux, &http2.Server{}))
			if err := http.ListenAndServe(addr, handler); err != nil {
				color.Red("connectrpc GitHub Automation service listen error: %s", err)
			}
		}()
	}
}
