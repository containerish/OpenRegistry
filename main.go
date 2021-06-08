package main

import (
	"os"

	"github.com/fatih/color"
	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/server"
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

	l := setupLogger()
	srv := server.NewServer(l, config)

	var errSig chan error
	errSig <- srv.Start()

	color.Yellow("docker registry server start error: %s", <-errSig)

	errSig <- srv.Stop()

	color.Yellow("docker registry server stopped: %s", <-errSig)
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	l := zerolog.New(os.Stdout)
	l.With().Caller().Logger()

	return l
}
