package cmd

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containerish/OpenRegistry/store/v2/migrations"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

func NewMigrationsCommand() *cli.Command {
	return &cli.Command{
		Name:    "migrations",
		Aliases: []string{"m"},
		Usage:   "Run database migrations for OpenRegistry data store",
		Subcommands: []*cli.Command{
			newDatabaseInitCommand(),
			newMigrationsRunCommand(),
			newMigrationsRollbackCommand(),
			newMigrationsGenrateCommand(),
			newDatabaseResetCommand(),
		},
		Action: func(ctx *cli.Context) error {
			return nil
		},
	}
}

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
			connector := getOpenRegistryConnector(ctx)
			db := getOpenRegistryDB(connector)
			migrations.PerformMigrations(ctx.Context, db)
			return nil
		},
	}

}

func newMigrationsRollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "Rollback the latest migrations",
		Action: func(ctx *cli.Context) error {
			return nil
		},
	}
}

func newDatabaseInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialise the database, create tables, roles, indexes, etc",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "openregistry-db-dsn",
				DefaultText: "postgres://localhost:5432/postgres",
				Value:       "postgres://localhost:5432/postgres",
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "admin-db-dsn",
				Value:       "postgres://localhost:5432/open_registry",
				DefaultText: "postgres://localhost:5432/open_registry",
				Required:    true,
			},
		},
		Action: func(ctx *cli.Context) error {
			adminConnector := getAdminConnector(ctx)
			adminDB := getAdminBunDB(adminConnector)
			_, err := adminDB.Exec("create database open_registry")
			if err != nil && !strings.Contains(err.Error(), "SQLSTATE=42P04") {
				return err
			}

			openregistryConnector := getOpenRegistryConnector(ctx)
			openRegistryDB := getOpenRegistryDB(openregistryConnector)

			migrator := migrations.NewMigrator(openRegistryDB)
			if err = migrator.Init(ctx.Context); err != nil {
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

			_, err = openRegistryDB.
				ExecContext(
					ctx.Context,
					"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ?",
					bun.Ident(openregistryConnector.Config().User),
				)
			if err != nil {
				return errors.New(
					color.RedString("Action=GrantPrivleges Created=❌ Error=%s", err),
				)
			}
			color.Green(`Action "GrantPrivleges" succeeded ✔︎`)

			return nil
		},
	}
}

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
			connector := getOpenRegistryConnector(ctx)
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

func newDatabaseResetCommand() *cli.Command {
	return &cli.Command{
		Name:    "reset",
		Aliases: []string{"re"},
		Usage:   "Re-initialise the database, first delete everything & then create tables, roles, indexes, etc",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "openregistry-db-dsn",
				Value:    "postgres://localhost:5432/open_registry",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			connector := getOpenRegistryConnector(ctx)
			db := getOpenRegistryDB(connector)

			return db.ResetModel(
				ctx.Context,
				&types.ContainerImageRepository{},
				&types.ContainerImageLayer{},
				&types.ImageManifest{},
				&types.User{},
				&types.Session{},
				&types.WebauthnSession{},
				&types.WebauthnCredential{},
				&types.Email{},
			)
		},
	}
}

func getOpenRegistryConnector(ctx *cli.Context) *pgdriver.Connector {
	dsn := ctx.String("openregistry-db-dsn")
	return pgdriver.NewConnector(pgdriver.WithDSN(dsn), pgdriver.WithInsecure(true))
}

func getAdminConnector(ctx *cli.Context) *pgdriver.Connector {
	dsn := ctx.String("admin-db-dsn")
	return pgdriver.NewConnector(pgdriver.WithDSN(dsn), pgdriver.WithInsecure(true))
}

func getOpenRegistryDB(connector *pgdriver.Connector) *bun.DB {
	sqlDB := sql.OpenDB(connector)
	bunWrappedDB := bun.NewDB(sqlDB, pgdialect.New())
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to database: %s", err)
		os.Exit(1100)
	}
	bunWrappedDB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithVerbose(true),
		bundebug.FromEnv("BUNDEBUG"),
	))

	return bunWrappedDB
}

func getAdminBunDB(connector *pgdriver.Connector) *bun.DB {
	sqlDB := sql.OpenDB(connector)
	bunWrappedDB := bun.NewDB(sqlDB, pgdialect.New())
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to database: %s", err)
		os.Exit(1100)
	}
	bunWrappedDB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithVerbose(true),
		bundebug.FromEnv("BUNDEBUG"),
	))

	return bunWrappedDB
}

func createOpenRegistryTables(ctx *cli.Context, db *bun.DB) error {
	_, err := db.NewCreateTable().Model(&types.User{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=users Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "users" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.ContainerImageLayer{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=layers Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "layers" created ✔︎`)

	_, err = db.
		NewCreateTable().
		Model(&types.ContainerImageRepository{}).
		Table().
		IfNotExists().
		Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=repositories Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "repositories" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.ImageManifest{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=image_manifests Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "image_manifests" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.Session{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=sessions Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "sessions" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.WebauthnSession{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=webauthn_sessions Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "webauthn_sessions" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.WebauthnCredential{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=webauthn_credentials Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "webauthn_credentials" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.Email{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=emails Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "emails" created ✔︎`)

	return nil
}
