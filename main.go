package main

import (
	"net/http"
	"os"

	"github.com/fatih/color"
	"github.com/jay-dee7/parachute/cache"
	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/server/registry/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

func main() {
	var configPath string
	if len(os.Args) != 2 {
		configPath = "./"
	}

	config, err := config.Load(configPath)
	if err != nil {
		color.Red("error reading config file: %s", err.Error())
		os.Exit(1)
	}

	color.Green("config: %s", config)

	var errSig chan error
	e := echo.New()

	l := setupLogger()
	localCache, err := cache.New("/tmp/badger")
	if err != nil {
		l.Err(err).Send()
		return
	}

	reg, err := registry.NewRegistry(l, localCache, e.Logger)
	if err != nil {
		l.Err(err).Send()
		return
	}

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(echo.Context) bool {
			return false
		},
		Format:           "method=${method}, uri=${uri}, status=${status} error=${error} latency=${latency} bytes_in=${bytes_in} bytes_out=${bytes_out} range=${Content-Range}\n",
		CustomTimeFormat: "",
		Output:           nil,
	}))

	e.Use(middleware.Recover())

	router := e.Group("/v2/:namespace")

	// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	router.Add(http.MethodPut, "/blobs/uploads/:uuid", reg.MonolithicUpload)

	// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	router.Add(http.MethodPut, "/blobs/uploads/:uuid", reg.CompleteUpload)

	// HEAD /v2/<name>/blobs/<digest>
	router.Add(http.MethodHead, "/blobs/:reference", reg.ManifestExists) // (LayerExists) should be called reference/digest

	// HEAD /v2/<name>/manifests/<reference>
	router.Add(http.MethodHead, "/manifests/:reference", reg.ManifestExists) //should be called reference/digest

	// PATCH /v2/<name>/blobs/uploads/<uuid>
	router.Add(http.MethodPatch, "/blobs/uploads/:uuid", reg.ChunkedUpload)

	// GET /v2/<name>/manifests/<reference>
	router.Add(http.MethodGet, "/manifests/:reference", reg.PullManifest)

	// PUT /v2/<name>/manifests/<reference>
	router.Add(http.MethodPut, "/manifests/:reference", reg.PushManifest)

	// GET /v2/<name>/blobs/<digest>
	router.Add(http.MethodGet, "/manifests/:digest", reg.PullLayer)

	// POST /v2/<name>/blobs/uploads/
	router.Add(http.MethodPost, "/blobs/uploads/", reg.StartUpload)

	// POST /v2/<name>/blobs/uploads/
	router.Add(http.MethodPost, "/blobs/uploads/:uuid", reg.PushLayer)

	e.Add(http.MethodGet, "/v2/", reg.ApiVersion)

	e.Start(config.Address())

// 	go func() {
// 		if err := e.Start(config.Address()); err != nil && err != http.ErrServerClosed {
// 			e.Logger.Fatal("shutting down the server")
// 		}
// 	}()

// 	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
// 	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
// 	quit := make(chan os.Signal, 1)
// 	signal.Notify(quit, os.Interrupt)
// 	<-quit
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	if err := e.Shutdown(ctx); err != nil {
// 		e.Logger.Fatal(err)
// 	}

	color.Yellow("docker registry server stopped: %s", <-errSig)
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	l := zerolog.New(os.Stdout)
	l.With().Caller().Logger()

	return l
}
