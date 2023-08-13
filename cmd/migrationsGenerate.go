package cmd

import (
	"fmt"

	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/fatih/color"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

func newMigrationsGenrateCommand() *cli.Command {
	return &cli.Command{
		Name:    "generate",
		Usage:   "Generate database migration files",
		Aliases: []string{"gen", "ge"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "openregistry-db-dsn",
				Value:    "postgres://localhost:5432/open_registry",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Value:    "column_name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "operation",
				Value:    "add",
				Required: true,
				Aliases:  []string{"op"},
				Usage:    "What will this migration do? Add column/s or remove them?",
			},
		},
		Action: func(ctx *cli.Context) error {
			name := ctx.String("name")
			operation := ctx.String("operation")
			opts := parseDatabaseFlags(ctx)
			connector := getDBConnectorFromCtx(false, opts)
			db := getOpenRegistryDB(connector)
			migrator := migrations.NewMigrator(db)

			migrationFile, err := migrator.CreateGoMigration(
				ctx.Context,
				fmt.Sprintf("%s_%s", operation, name),
				migrate.WithPackageName("migrations"),
			)
			if err != nil {
				return err
			}
			color.Green("Migration file created at - \"%s\" ✔︎", migrationFile.Path)
			return nil
		},
	}
}
