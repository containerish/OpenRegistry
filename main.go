package main

import (
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/server"
	"github.com/rs/zerolog"
)
func main() {

	path := "./"
	config, err := config.Load(path)
	if err != nil {
		color.Red("error reading config file: %s", err.Error())
		os.Exit(1)
	}

	l := setupLogger()
	srv := server.NewServer(l, config)
	defer srv.Stop()

	log.Fatalln(srv.Start())
}

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	l := zerolog.New(os.Stdout)
	l.With().Caller().Logger()

	return l
}
