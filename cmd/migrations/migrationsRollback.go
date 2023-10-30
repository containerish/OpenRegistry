package migrations

import (
	"errors"

	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func newMigrationsRollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback the latest migrations",
		Flags: getOpenRegistryDatabaseCmdFlags(),
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			connector := getOpenRegistryDBConnectorFromCtx(opts)
			db := getOpenRegistryDB(connector)

			migrator := migrations.NewMigrator(db)

			if err := migrator.Lock(ctx.Context); err != nil {
				return errors.New(
					color.RedString("Action=migration_lock Created=❌ Error=%s", err),
				)

			}
			defer func() {
				_ = migrator.Unlock(ctx.Context)
			}()

			group, err := migrator.Rollback(ctx.Context)
			if err != nil {
				return errors.New(
					color.RedString("Action=migration_rollback Created=❌ Error=%s", err),
				)
			}

			color.Green(`Migration rollback with "ID=%d" done ✔︎`, group.ID)
			return nil
		},
	}
}
