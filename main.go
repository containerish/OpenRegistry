package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/containerish/OpenRegistry/auth"
	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs/client"
	healthchecks "github.com/containerish/OpenRegistry/health-checks"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/containerish/OpenRegistry/router"
	github_actions_server "github.com/containerish/OpenRegistry/services/kon/github_actions/v1/server"
	"github.com/containerish/OpenRegistry/store/postgres"
	build_automation_store "github.com/containerish/OpenRegistry/store/postgres/build_automation"
	"github.com/containerish/OpenRegistry/telemetry"
	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	cfg, err := config.ReadYamlConfig()
	if err != nil {
		color.Red("error reading cfg file: %s", err.Error())
		os.Exit(1)
	}
	e := echo.New()

	pgStore, err := postgres.New(&cfg.StoreConfig)
	if err != nil {
		color.Red("ERR_PG_CONN: %s", err.Error())
		return
	}
	defer pgStore.Close()

	buildAutomationStore, err := build_automation_store.New(&cfg.StoreConfig)
	if err != nil {
		color.Red("ERR_BUILD_AUTOMATION_PG_CONN: %s", err.Error())
		return
	}
	defer buildAutomationStore.Close()

	fluentBitCollector, err := fluentbit.New(cfg)
	if err != nil {
		color.Red("error initializing fluentbit collector: %s\n", err)
		os.Exit(1)
	}

	logger := telemetry.ZLogger(fluentBitCollector, cfg.Environment)
	authSvc := auth.New(cfg, pgStore, logger)
	webauthnServer := auth_server.NewWebauthnServer(cfg, pgStore, logger)
	healthCheckHandler := healthchecks.NewHealthChecksAPI(pgStore)

	dfs := client.NewDFSBackend(cfg.Environment, cfg.Endpoint(), &cfg.DFS)
	reg, err := registry.NewRegistry(pgStore, dfs, logger, cfg)
	if err != nil {
		e.Logger.Errorf("error creating new container registry: %s", err)
		return
	}

	ext, err := extensions.New(pgStore, logger)
	if err != nil {
		e.Logger.Errorf("error creating new container registry extensions api: %s", err)
		return
	}

	router.Register(cfg, e, reg, authSvc, webauthnServer, ext)
	router.RegisterHealthCheckEndpoint(e, healthCheckHandler)
	if cfg.Integrations.GetGithubConfig() != nil && cfg.Integrations.GetGithubConfig().Enabled {
		ghApp, err := github.NewGithubApp(
			cfg.Integrations.GetGithubConfig(),
			pgStore,
			logger,
			cfg.WebAppConfig.AllowedEndpoints,
			cfg.Environment,
		)
		if err != nil {
			e.Logger.Errorf("error initializing Github App Service: %s", err)
			return
		}

		ghApp.RegisterRoutes(e.Group("/github"))
		ghConfig := cfg.Integrations.GetGithubConfig()
		githubMux := github_actions_server.NewGithubActionsServer(
			ghConfig,
			&cfg.Registry.Auth,
			logger,
			buildAutomationStore,
			pgStore,
		)
		go func() {
			hostPort := fmt.Sprintf("%s:%d", ghConfig.Host, ghConfig.Port)
			color.Green("connect-go gRPC running on: %s", hostPort)
			if err := http.ListenAndServe(hostPort, h2c.NewHandler(githubMux, &http2.Server{})); err != nil {
				color.Red("gRPC listen error: %s", err)
			}
		}()
	}

	color.Red("error initialising OpenRegistry Server: %s", buildHTTPServer(cfg, e))
}

func buildHTTPServer(cfg *config.OpenRegistryConfig, e *echo.Echo) error {
	color.Green("Environment: %s", cfg.Environment)
	color.Green("Service Endpoint: %s\n", cfg.Endpoint())
	// for this to work, we need a custom http serve
	if cfg.Registry.TLS.Enabled {
		return e.StartTLS(cfg.Registry.Address(), cfg.Registry.TLS.PubKey, cfg.Registry.TLS.PrivateKey)
	}

	return e.Start(cfg.Registry.Address())
}
