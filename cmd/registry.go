package cmd

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
	"github.com/containerish/OpenRegistry/store/v2/emails"
	store_v2 "github.com/containerish/OpenRegistry/store/v2/registry/postgres"
	"github.com/containerish/OpenRegistry/store/v2/sessions"
	"github.com/containerish/OpenRegistry/store/v2/users"
	"github.com/containerish/OpenRegistry/store/v2/webauthn"
	"github.com/containerish/OpenRegistry/telemetry"
	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func NewRegistryCommand() *cli.Command {
	return &cli.Command{
		Name:    "start",
		Aliases: []string{"s"},
		Usage:   "start the OpenRegistry server",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "daemon", Value: false, Usage: "Run the OpenRegistry server in background"},
			&cli.StringFlag{
				Name:      "config-file",
				Value:     "$HOME/.openregistry/config.yaml",
				Usage:     "Path to the OpenRegistry config file",
				FilePath:  "$HOME/.openregistry/config.yaml",
				TakesFile: true,
				Aliases:   []string{"c"},
			},
			&cli.StringFlag{Name: "log-format", Value: "pretty", Usage: "One of: pretty, json"},
			&cli.StringFlag{Name: "log-level", Value: "info", Usage: "One of: info, debug"},
		},
		Action: func(ctx *cli.Context) error {
			RunRegistryServer(ctx)
			return nil
		},
	}
}

func RunRegistryServer(ctx *cli.Context) {
	configPath := ctx.String("config")
	cfg, err := config.ReadYamlConfig(configPath)
	if err != nil {
		color.Red("error reading cfg file: %s", err.Error())
		os.Exit(1)
	}

	fluentBitCollector, err := fluentbit.New(cfg)
	if err != nil {
		color.Red("error initializing fluentbit collector: %s\n", err)
		os.Exit(1)
	}

	logger := telemetry.ZLogger(fluentBitCollector, cfg.Environment)
	e := echo.New()

	pgStore, err := postgres.New(&cfg.StoreConfig)
	if err != nil {
		color.Red("ERR_PG_CONN: %s", err.Error())
		return
	}
	defer pgStore.Close()
	_ = pgStore

	pgStoreV2 := store_v2.NewStore(cfg.StoreConfig.Endpoint(), 5, logger, cfg.Environment)
	usersStore := users.NewStore(pgStoreV2.DB(), logger)
	sessionsStore := sessions.NewStore(pgStoreV2.DB())
	webauthnStore := webauthn.NewStore(pgStoreV2.DB())
	emailStore := emails.NewStore(pgStoreV2.DB())

	buildAutomationStore, err := build_automation_store.New(&cfg.StoreConfig)
	if err != nil {
		color.Red("ERR_BUILD_AUTOMATION_PG_CONN: %s", err.Error())
		return
	}
	defer buildAutomationStore.Close()

	authSvc := auth.New(cfg, usersStore, sessionsStore, emailStore, logger)
	webauthnServer := auth_server.NewWebauthnServer(cfg, webauthnStore, sessionsStore, usersStore, logger)
	healthCheckHandler := healthchecks.NewHealthChecksAPI(pgStoreV2)

	dfs := client.NewDFSBackend(cfg.Environment, cfg.Endpoint(), &cfg.DFS)
	reg, err := registry.NewRegistry(pgStoreV2, dfs, logger, cfg)
	if err != nil {
		e.Logger.Errorf("error creating new container registry: %s", err)
		return
	}

	ext, err := extensions.New(pgStoreV2, logger)
	if err != nil {
		e.Logger.Errorf("error creating new container registry extensions api: %s", err)
		return
	}

	router.Register(cfg, e, reg, authSvc, webauthnServer, ext, pgStoreV2)
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
