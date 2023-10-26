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
		Name:     "generate",
		Usage:    "Generate database migration files",
		Aliases:  []string{"gen", "ge"},
		Category: CategoryMigrations,
		Flags:    append(getOpenRegistryDatabaseCmdFlags(), getMigrationGenerateCmdFlags()...),
		Action:   migrationGenerateCmd,
	}
}

func migrationGenerateCmd(ctx *cli.Context) error {
	opts := parseDatabaseFlags(ctx)
	openRegistryDB, err := createOpenRegistryDatabase(ctx, opts)
	if err != nil {
		return err
	}
	name := ctx.String("name")
	operation := ctx.String("operation")
	migrator := migrations.NewMigrator(openRegistryDB)

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
}

func getMigrationGenerateCmdFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}
