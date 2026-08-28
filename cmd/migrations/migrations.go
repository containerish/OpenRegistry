package migrations

import (
	"crypto/tls"
	"database/sql"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/urfave/cli/v2"
)

func NewMigrationsCommand() *cli.Command {
	return &cli.Command{
		Name:    "migrations",
		Aliases: []string{"m"},
		Usage:   "Run database migrations for OpenRegistry data store",
		Description: `Perform migrations for the OpenRegistry database like database initialisation, migrations, 
rollback, reset, etc`,
		UsageText: `OpenRegistry CLI provides a collection of commands for running database migrations.
Examples:
    1. openregistry migrations init --openregistry-db-dsn=<openregistry_db_dsn> --admin-db-dsn=<admin_db_dsn>
    2. openregistry migrations run --openregistry-db-dsn=<openregistry_db_dsn> 
    3. openregistry migrations generate --openregistry-db-dsn=<openregistry_db_dsn> 
    4. openregistry migrations reset --openregistry-db-dsn=<openregistry_db_dsn>`,
		Subcommands: []*cli.Command{
			newDatabaseInitCommand(),
			newMigrationsRunCommand(),
			newMigrationsRollbackCommand(),
			newMigrationsGenrateCommand(),
			newDatabaseResetCommand(),
		},
		Action: nil,
	}
}

func getOpenRegistryDB(connector *pgdriver.Connector) *bun.DB {
	sqlDB := sql.OpenDB(connector)
	bunWrappedDB := bun.NewDB(sqlDB, pgdialect.New())
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to open_registry database: %s", err)
		os.Exit(1100)
	}

	return bunWrappedDB
}

func getAdminBunDB(connector *pgdriver.Connector) *bun.DB {
	sqlDB := sql.OpenDB(connector)
	bunWrappedDB := bun.NewDB(sqlDB, pgdialect.New())
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to admin (postgres) database: %s", err)
		os.Exit(1100)
	}

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
	_, err = db.NewCreateTable().Model(&types.RepositoryBuild{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=repository_builds Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "repository_builds" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.RepositoryBuildProject{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=repository_build_projects Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "repository_build_projects" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.Permissions{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=permissions Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "permissions" created ✔︎`)

	_, err = db.NewCreateTable().Model(&types.AuthTokens{}).Table().IfNotExists().Exec(ctx.Context)
	if err != nil {
		return errors.New(
			color.RedString("Table=auth_tokens Created=❌ Error=%s", err),
		)
	}
	color.Green(`Table "auth_tokens" created ✔︎`)

	return nil
}

func getAdminDBConnectorFromCtx(opts *databaseOptions) *pgdriver.Connector {
	if opts.adminDSN != "" {
		return pgdriver.NewConnector(pgdriver.WithDSN(opts.adminDSN), pgdriver.WithInsecure(opts.insecure))
	}

	return pgdriver.NewConnector(
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(opts.address),
		//nolint
		pgdriver.WithTLSConfig(&tls.Config{InsecureSkipVerify: opts.insecure}),
		pgdriver.WithInsecure(opts.insecure),
		pgdriver.WithUser(opts.adminUsername),
		pgdriver.WithTimeout(opts.timeout),
		pgdriver.WithDatabase(opts.adminDB),
		pgdriver.WithPassword(opts.adminPassword),
		pgdriver.WithApplicationName("OpenRegistry"),
	)
}

func getOpenRegistryDBConnectorFromCtx(opts *databaseOptions) *pgdriver.Connector {
	if opts.openRegistryDSN != "" {
		return pgdriver.NewConnector(pgdriver.WithDSN(opts.openRegistryDSN), pgdriver.WithInsecure(opts.insecure))
	}

	return pgdriver.NewConnector(
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(opts.address),
		//nolint
		pgdriver.WithTLSConfig(&tls.Config{InsecureSkipVerify: opts.insecure}),
		pgdriver.WithInsecure(opts.insecure),
		pgdriver.WithUser(opts.username),
		pgdriver.WithTimeout(opts.timeout),
		pgdriver.WithDatabase(opts.database),
		pgdriver.WithPassword(opts.password),
		pgdriver.WithApplicationName("OpenRegistry"),
	)
}

func createOpenRegistryDatabase(ctx *cli.Context, opts *databaseOptions) (*bun.DB, error) {
	adminConnector := getAdminDBConnectorFromCtx(opts)
	adminDB := getAdminBunDB(adminConnector)

	_, err := adminDB.ExecContext(
		ctx.Context,
		"CREATE USER ? WITH ENCRYPTED PASSWORD ?",
		bun.Ident(opts.username),
		opts.password,
	)
	if err != nil && !strings.Contains(err.Error(), "SQLSTATE=42710") {
		return nil, errors.New(
			color.RedString("Action=CreateUser Created=❌ Error=%s", err),
		)
	}

	_, err = adminDB.Exec("create database ? with owner = ?", bun.Ident(opts.database), opts.username)
	if err != nil && !strings.Contains(err.Error(), "SQLSTATE=42P04") {
		return nil, errors.New(
			color.RedString("Action=CreateDatabase Created=❌ Error=%s", err),
		)
	}

	_, err = adminDB.
		ExecContext(
			ctx.Context,
			"GRANT ALL PRIVILEGES ON DATABASE ? to ?",
			bun.Ident(opts.database),
			bun.Ident(opts.username),
		)
	if err != nil {
		return nil, errors.New(
			color.RedString("Action=GrantDBPrivleges Created=❌ Error=%s", err),
		)
	}
	color.Green(`Action "GrantDBPrivleges" succeeded ✔︎`)

	openregistryDB := getOpenRegistryDB(getOpenRegistryDBConnectorFromCtx(opts))
	_, err = adminDB.
		ExecContext(
			ctx.Context,
			"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ?",
			bun.Ident(opts.username),
		)
	if err != nil {
		return nil, errors.New(
			color.RedString("Action=GrantAll Created=❌ Error=%s", err),
		)
	}
	color.Green(`Action "GrantAllPrivleges" succeeded ✔︎`)
	return openregistryDB, nil
}

type databaseOptions struct {
	address         string
	database        string
	username        string
	password        string
	adminDSN        string
	openRegistryDSN string
	adminDB         string
	adminUsername   string
	adminPassword   string
	timeout         time.Duration
	insecure        bool
}

func parseDatabaseFlags(ctx *cli.Context) *databaseOptions {
	opts := &databaseOptions{
		address:         net.JoinHostPort(ctx.String("host"), ctx.String("port")),
		database:        ctx.String("database"),
		username:        ctx.String("username"),
		password:        ctx.String("password"),
		timeout:         ctx.Duration("timeout"),
		insecure:        ctx.Bool("insecure"),
		openRegistryDSN: ctx.String("openregistry-db-dsn"),
		adminDSN:        ctx.String("admin-db-dsn"),
		adminDB:         ctx.String("admin-db"),
		adminUsername:   ctx.String("admin-db-username"),
		adminPassword:   ctx.String("admin-db-password"),
	}

	return opts
}

func getOpenRegistryDatabaseCmdFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "openregistry-db-dsn",
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

func getAdminDatabaseFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}
