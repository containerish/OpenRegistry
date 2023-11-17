package migrations

import (
	"github.com/containerish/OpenRegistry/store/v1/migrations"
	"github.com/urfave/cli/v2"
)

func newMigrationsRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Run any new migrations",
		Flags: getOpenRegistryDatabaseCmdFlags(),
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			connector := getOpenRegistryDBConnectorFromCtx(opts)
			db := getOpenRegistryDB(connector)
			migrations.PerformMigrations(ctx.Context, db)
			return nil
		},
	}

}
