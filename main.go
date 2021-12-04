package main

import (
	"os"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/router"
	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
)

func main() {
	cfg, err := config.LoadFromENV()
	if err != nil {
		color.Red("error reading cfg file: %s", err.Error())
		os.Exit(1)
	}

	e := echo.New()

	localCache, err := cache.New(".kvstore")
	if err != nil {
		e.Logger.Errorf("error opening local kv store: %s", err)
		return
	}
	defer localCache.Close()

	pgConfig, _ := config.NewStoreConfig()
	pgStore, err := postgres.New(pgConfig)
	if err != nil {
		color.Red("error here: %s", err.Error())
		return
	}

	fluentBitCollector, err := fluentbit.New(cfg)
	if err != nil {
		color.Red("error initializing fluentbit collector: %s\n", err)
		os.Exit(1)
	}

	logger := telemetry.ZLogger(telemetry.SetupLogger(), fluentBitCollector)
	authSvc := auth.New(localCache, cfg, pgStore, logger)
	skynetClient := skynet.NewClient(cfg)

	reg, err := registry.NewRegistry(skynetClient, localCache, logger, pgStore)
	if err != nil {
		e.Logger.Errorf("error creating new container registry: %s", err)
		return
	}

	router.Register(cfg, e, reg, authSvc, localCache, pgStore)
	logger.Errorf("error initialising OpenRegistry Server: %s", e.Start(cfg.Address()))
}
