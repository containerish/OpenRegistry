package migrations

import (
	"errors"
	"strings"

	"github.com/containerish/OpenRegistry/store/v1/migrations"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func newDatabaseInitCommand() *cli.Command {
	return &cli.Command{
		Name:   "init",
		Usage:  "Initialise the database, create tables, roles, indexes, etc",
		Flags:  append(getOpenRegistryDatabaseCmdFlags(), getAdminDatabaseFlags()...),
		Action: databaseInitCmd,
	}
}

func databaseInitCmd(ctx *cli.Context) error {
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
	if err != nil && !strings.Contains(err.Error(), "SQLSTATE=42710") {
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
}
