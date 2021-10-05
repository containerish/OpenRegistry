package main

import (
	"log"
	"os"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/router"
	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/telemetry"
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

	authSvc := auth.New(localCache, cfg)
	skynetClient := skynet.NewClient(cfg)

	l := telemetry.SetupLogger()
	reg, err := registry.NewRegistry(skynetClient, l, localCache, e.Logger)
	if err != nil {
		e.Logger.Errorf("error creating new container registry: %s", err)
		return
	}

	router.Register(e, reg, authSvc, localCache)
	log.Println(e.Start(cfg.Address()))
}
