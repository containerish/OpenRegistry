package main

import (
	"os"
	"time"

	"github.com/containerish/OpenRegistry/cmd"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "OpenRegistry",
		Usage: "OpenRegistry CLI",
		Authors: []*cli.Author{
			{
				Name:  "Containerish OSS Team",
				Email: "team@cntr.sh",
			},
		},
		Compiled: time.Now(),
		Description: `This CLI program can be used to manage an OpenRegistry instance.
You can perform actions such as datastore migrations, rollbacks, starting the registry server,
running OCI tests against the server, etc`,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "validateConfig", Value: false, Usage: "--validateConfig"},
		},
		Commands: []*cli.Command{
			cmd.NewMigrationsCommand(),
			cmd.NewRegistryCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}
}
