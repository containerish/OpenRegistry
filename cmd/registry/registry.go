package registry

import (
	"errors"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/urfave/cli/v2"

	user_api "github.com/containerish/OpenRegistry/api/users"
	"github.com/containerish/OpenRegistry/auth"
	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/containerish/OpenRegistry/config"
	dfs_client "github.com/containerish/OpenRegistry/dfs/client"
	healthchecks "github.com/containerish/OpenRegistry/health-checks"
	"github.com/containerish/OpenRegistry/orgmode"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/containerish/OpenRegistry/router"
	store_v2 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/automation"
	"github.com/containerish/OpenRegistry/store/v1/emails"
	"github.com/containerish/OpenRegistry/store/v1/permissions"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/sessions"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/store/v1/webauthn"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/telemetry/otel"
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
	// this helps with performance but isn't super safe but it's okay in our case since we're not doing anything super
	// secure with uuids anyway
	uuid.EnableRandPool()

	configPath := ctx.String("config")
	cfg, err := config.ReadYamlConfig(configPath)
	if err != nil {
		return errors.New(color.RedString("error reading cfg file: %s", err.Error()))
	}

	logger := telemetry.ZeroLogger(cfg.Environment, cfg.Telemetry)
	dfs := dfs_client.New(cfg.Environment, cfg.Endpoint(), &cfg.DFS, logger)
	rawDB := store_v2.New(cfg.StoreConfig, cfg.Environment)
	defer rawDB.Close()

	registryStore := registry_store.New(rawDB, logger)
	usersStore := users.New(rawDB, logger)
	sessionsStore := sessions.New(rawDB)
	webauthnStore := webauthn.New(rawDB)
	emailStore := emails.New(rawDB)
	permissionsStore := permissions.New(rawDB, logger)
	automationStore := automation.New(rawDB, logger)

	authApi := auth.New(cfg, usersStore, sessionsStore, emailStore, registryStore, permissionsStore, logger)
	webauthnApi := auth_server.NewWebauthnServer(cfg, webauthnStore, sessionsStore, usersStore, logger)
	healthCheckApi := healthchecks.NewHealthChecksAPI(&store_v2.DBPinger{DB: rawDB})
	usersApi := user_api.NewApi(usersStore, logger)
	registryApi := registry.NewRegistry(registryStore, dfs, logger, cfg)
	extensionsApi := extensions.New(registryStore, logger)
	orgApi := orgmode.New(permissionsStore, usersStore, logger)

	baseRouter := router.Register(
		cfg,
		logger,
		registryApi,
		authApi,
		webauthnApi,
		extensionsApi,
		orgApi,
		usersApi,
		healthCheckApi,
		registryStore,
		usersStore,
		automationStore,
		dfs,
	)

	otelShutdownFunc := otel.ConfigureOtel(cfg.Telemetry.Honeycomb, "openregistry-api", baseRouter)
	if otelShutdownFunc != nil {
		defer otelShutdownFunc()
	}

	if err = buildHTTPServer(cfg, baseRouter); err != nil {
		return errors.New(color.RedString("error initialising OpenRegistry Server: %s", err))
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
