package registry

import (
	"fmt"
	"net/http"

	"github.com/containerish/OpenRegistry/auth"
	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs/client"
	healthchecks "github.com/containerish/OpenRegistry/health-checks"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/containerish/OpenRegistry/router"
	github_actions_server "github.com/containerish/OpenRegistry/services/kon/github_actions/v1/server"
	store_v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/automation"
	"github.com/containerish/OpenRegistry/store/v2/emails"
	registry_store "github.com/containerish/OpenRegistry/store/v2/registry"
	"github.com/containerish/OpenRegistry/store/v2/sessions"
	"github.com/containerish/OpenRegistry/store/v2/users"
	"github.com/containerish/OpenRegistry/store/v2/webauthn"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/telemetry/otel"
	"github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// const CategoryOpenRegistry = "OpenRegistry"

func NewRegistryCommand() *cli.Command {
	return &cli.Command{
		Name:    "start",
		Aliases: []string{"s"},
		Usage:   "start the OpenRegistry server",
		// Category: CategoryOpenRegistry,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "config-file",
				Value:     "$HOME/.openregistry/config.yaml",
				Usage:     "Path to the OpenRegistry config file",
				FilePath:  "$HOME/.openregistry/config.yaml",
				TakesFile: true,
				Aliases:   []string{"c"},
			},
			&cli.BoolFlag{Name: "daemon", Value: false, Usage: "Run the OpenRegistry server in background"},
			&cli.StringFlag{Name: "log-format", Value: "pretty", Usage: "One of: pretty, json"},
			&cli.StringFlag{Name: "log-level", Value: "info", Usage: "One of: info, debug"},
		},
		Action: RunRegistryServer,
	}
}

func RunRegistryServer(ctx *cli.Context) error {
	configPath := ctx.String("config")
	cfg, err := config.ReadYamlConfig(configPath)
	if err != nil {
		return fmt.Errorf(color.RedString("error reading cfg file: %s", err.Error()))
	}

	logger := telemetry.ZLogger(cfg.Environment, cfg.Telemetry)
	e := echo.New()

	rawDB := store_v2.NewDB(cfg.StoreConfig, cfg.Environment)
	registryStore := registry_store.NewStore(rawDB, logger)
	usersStore := users.NewStore(rawDB, logger)
	sessionsStore := sessions.NewStore(rawDB)
	webauthnStore := webauthn.NewStore(rawDB)
	emailStore := emails.NewStore(rawDB)

	buildAutomationStore, err := automation.New(rawDB, logger)
	if err != nil {
		return fmt.Errorf(color.RedString("ERR_BUILD_AUTOMATION_PG_CONN: %s", err.Error()))
	}
	defer buildAutomationStore.Close()

	authSvc := auth.New(cfg, usersStore, sessionsStore, emailStore, logger)
	webauthnServer := auth_server.NewWebauthnServer(cfg, webauthnStore, sessionsStore, usersStore, logger)
	healthCheckHandler := healthchecks.NewHealthChecksAPI(&store_v2.DBPinger{DB: rawDB})

	dfs := client.NewDFSBackend(cfg.Environment, cfg.Endpoint(), &cfg.DFS)
	reg, err := registry.NewRegistry(registryStore, dfs, logger, cfg)
	if err != nil {
		return fmt.Errorf(color.RedString("error initializing registry services: %s", err))
	}

	ext, err := extensions.New(registryStore, logger)
	if err != nil {
		return fmt.Errorf(color.RedString("error creating new container registry extensions api: %s", err))
	}

	router.Register(cfg, e, reg, authSvc, webauthnServer, ext, registryStore)
	router.RegisterHealthCheckEndpoint(e, healthCheckHandler)

	if cfg.Integrations.GetGithubConfig() != nil && cfg.Integrations.GetGithubConfig().Enabled {
		ghApp, ghErr := github.NewGithubApp(
			cfg.Integrations.GetGithubConfig(),
			usersStore,
			logger,
			cfg.WebAppConfig.AllowedEndpoints,
			cfg.Environment,
		)
		if ghErr != nil {
			return fmt.Errorf(color.RedString("error initializing Github App Service: %s", ghErr))
		}

		ghApp.RegisterRoutes(e.Group("/github"))
		ghConfig := cfg.Integrations.GetGithubConfig()
		githubMux := github_actions_server.NewGithubActionsServer(
			ghConfig,
			&cfg.Registry.Auth,
			logger,
			buildAutomationStore,
			usersStore,
		)
		go func() {
			hostPort := fmt.Sprintf("%s:%d", ghConfig.Host, ghConfig.Port)
			color.Green("connect-go gRPC running on: %s", hostPort)
			if err = http.ListenAndServe(hostPort, h2c.NewHandler(githubMux, &http2.Server{})); err != nil {
				color.Red("gRPC listen error: %s", err)
			}
		}()
	}

	otelShutdownFunc := otel.ConfigureOtel(cfg.Telemetry, "openregistry-api", e)
	if otelShutdownFunc != nil {
		defer otelShutdownFunc()
	}

	if err = buildHTTPServer(cfg, e); err != nil {
		return fmt.Errorf(color.RedString("error initialising OpenRegistry Server: %s", err))
	}

	return nil
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
