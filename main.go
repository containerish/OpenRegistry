package main

import (
	"os"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs/filebase"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/containerish/OpenRegistry/router"
	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
)

func main() {
	cfg, err := config.ReadYamlConfig()
	if err != nil {
		color.Red("error reading cfg file: %s", err.Error())
		os.Exit(1)
	}
	e := echo.New()

	pgStore, err := postgres.New(cfg.StoreConfig)
	if err != nil {
		color.Red("ERR_PG_CONN: %s", err.Error())
		return
	}
	defer pgStore.Close()

	fluentBitCollector, err := fluentbit.New(cfg)
	if err != nil {
		color.Red("error initializing fluentbit collector: %s\n", err)
		os.Exit(1)
	}

	logger := telemetry.ZLogger(fluentBitCollector, cfg.Environment)
	authSvc := auth.New(cfg, pgStore, logger)
	skynetClient := skynet.NewClient(cfg)

	filebase := filebase.New(cfg.DFS.S3Any)
	reg, err := registry.NewRegistry(skynetClient, logger, pgStore, filebase)
	if err != nil {
		e.Logger.Errorf("error creating new container registry: %s", err)
		return
	}

	ext, err := extensions.New(pgStore, logger)
	if err != nil {
		e.Logger.Errorf("error creating new container registry extensions api: %s", err)
		return
	}

	router.Register(cfg, e, reg, authSvc, ext)
	color.Red("error initialising OpenRegistry Server: %s", buildHTTPServer(cfg, e))
}

func buildHTTPServer(cfg *config.OpenRegistryConfig, e *echo.Echo) error {
	color.Green("Environment: %s", cfg.Environment)
	color.Green("Service Endpoint: %s\n", cfg.Endpoint())
	// for this to work, we need a custom http serve
	// if cfg.Environment == config.Prod {
	// 	return e.StartTLS(cfg.Registry.Address(), cfg.Registry.TLS.PubKey, cfg.Registry.TLS.PrivateKey)
	// }

	return e.Start(cfg.Registry.Address())
}
