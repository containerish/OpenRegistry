package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/containerish/OpenRegistry/cmd/extras"
	"github.com/containerish/OpenRegistry/cmd/migrations"
	"github.com/containerish/OpenRegistry/cmd/registry"
	"github.com/urfave/cli/v2"
)

var (
	//nolint
	GitCommit string
	//nolint
	Version string
)

func main() {
	var (
		projectAuthors = []*cli.Author{
			{
				Name:  "Containerish OSS Team",
				Email: "team@cntr.sh",
			},
		}

		rootCmdFlags = []cli.Flag{
			&cli.BoolFlag{Name: "validateConfig", Value: false, Usage: "--validateConfig"},
		}

		commands = []*cli.Command{
			migrations.NewMigrationsCommand(),
			registry.NewRegistryCommand(),
			extras.NewExtrasCommand(),
		}
	)

	const (
		rootCmdDescription = `This CLI program can be used to manage an OpenRegistry instance.
You can perform actions such as datastore migrations, rollbacks, starting the registry server,
running OCI tests against the server, etc`
		cliName = "OpenRegistry"
		usage   = cliName
	)

	app := &cli.App{
		Name:                   cliName,
		Usage:                  usage,
		Authors:                projectAuthors,
		UseShortOptionHandling: true,
		Suggest:                true,
		Version:                renderVersion(),
		Description:            rootCmdDescription,
		Flags:                  rootCmdFlags,
		Commands:               commands,
	}

	if err := app.RunContext(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func renderVersion() string {
	if !strings.HasPrefix(Version, "v") {
		Version = "v" + Version
	}
	return fmt.Sprintf(`Version: %s Commit: %s`, Version, GitCommit)
}
