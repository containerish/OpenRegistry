package cmd

import (
	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/urfave/cli/v2"
)

func newMigrationsRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Run any new migrations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "openregistry-db-dsn",
				DefaultText: "postgres://localhost:5432/open_registry",
				Value:       "postgres://localhost:5432/open_registry",
				Required:    true,
			},
		},
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			connector := getDBConnectorFromCtx(false, opts)
			db := getOpenRegistryDB(connector)
			migrations.PerformMigrations(ctx.Context, db)
			return nil
		},
	}

}
