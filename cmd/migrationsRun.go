package cmd

import (
	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/urfave/cli/v2"
)

func newMigrationsRunCommand() *cli.Command {
	return &cli.Command{
		Name:     "run",
		Usage:    "Run any new migrations",
		Flags:    getOpenRegistryDatabaseCmdFlags(),
		Category: CategoryMigrations,
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			connector := getDBConnectorFromCtx(false, opts)
			db := getOpenRegistryDB(connector)
			migrations.PerformMigrations(ctx.Context, db)
			return nil
		},
	}

}
