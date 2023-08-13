package cmd

import (
	"errors"
	"time"

	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func newDatabaseInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialise the database, create tables, roles, indexes, etc",
		Flags: getInitCmdFlags(),
		Action: func(ctx *cli.Context) error {
			opts := parseDatabaseFlags(ctx)
			openRegistryDB, err := createOpenRegistryDatabase(ctx, opts)
			if err != nil {
				return err
			}

			migrator := migrations.NewMigrator(openRegistryDB)
			err = migrator.Init(ctx.Context)
			if err != nil {
				return errors.New(
					color.RedString("Tables=migration_locks Created=❌ Error=%s", err),
				)
			}
			color.Green(`Table "open_registry_migration_locks" created ✔︎`)
			color.Green(`Table "open_registry_migrations" created ✔︎`)

			if err = createOpenRegistryTables(ctx, openRegistryDB); err != nil {
				return err
			}

			_, err = openRegistryDB.ExecContext(
				ctx.Context,
				"alter table repositories add constraint fk_owner_id foreign key (owner_id) references users(id)",
			)
			if err != nil {
				return errors.New(
					color.RedString("Table=repositories Alter=❌ Error=%s", err),
				)
			}
			color.Green(`Alter "owner_id" to add FK constraint done ✔︎`)

			_, err = openRegistryDB.ExecContext(
				ctx.Context,
				"alter table image_manifests add unique (reference,repository_id)",
			)
			if err != nil {
				return errors.New(
					color.RedString("Table=image_manifests Alter=❌ Error=%s", err),
				)
			}
			color.Green(`Alter "reference,repository_id" to add unique group constraint done ✔︎`)

			return nil
		},
	}
}

func getInitCmdFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "openregistry-db-dsn",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "admin-db-dsn",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "admin-db",
			Value:    "postgres",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "admin-db-username",
			Value:    "postgres",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "admin-db-password",
			Value:    "",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "database",
			Value:    "open_registry",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "host",
			Value:    "0.0.0.0",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "port",
			Value:    "5432",
			Required: false,
		},
		&cli.StringFlag{
			Name:        "username",
			Value:       "open_registry_user",
			DefaultText: "open_registry_user",
			Required:    false,
		},
		&cli.StringFlag{
			Name:     "password",
			Value:    "",
			Required: false,
		},
		&cli.DurationFlag{
			Name:     "timeout",
			Value:    time.Second * 60,
			Required: false,
		},
		&cli.BoolFlag{
			Name:     "insecure",
			Value:    false,
			Required: false,
		},
	}
}
