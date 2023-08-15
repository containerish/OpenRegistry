package main

import (
	"fmt"
	"log"
	"os"

	"github.com/containerish/OpenRegistry/cmd"
	"github.com/urfave/cli/v2"
)

var (
	//nolint
	GitCommit string
	//nolint
	Version string
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
		Metadata: map[string]interface{}{
			"something": "here",
		},
		UseShortOptionHandling: true,
		Suggest:                true,
		Version:                renderVersion(),
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
		log.Fatal(err)
	}
}

func renderVersion() string {
	return fmt.Sprintf(`
Version: %s
Commit: %s`, Version, GitCommit)
}
