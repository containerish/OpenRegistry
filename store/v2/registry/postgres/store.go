package postgres

import (
	"context"
	"database/sql"
	"os"
	"strings"

	"github.com/containerish/OpenRegistry/config"
	registry_store_v2 "github.com/containerish/OpenRegistry/store/v2/registry"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/schema"
)

type registryStore struct {
	rawDB  *sql.DB
	db     *bun.DB
	logger telemetry.Logger
}

func (rs *registryStore) Ping(ctx context.Context) error {
	return rs.db.PingContext(ctx)
}

func NewStore(dsn string, maxOpenConns int, logger telemetry.Logger, env config.Environment) registry_store_v2.RegistryStore {
	var (
		dialect schema.Dialect
		sqlDB   *sql.DB
	)
	if strings.HasPrefix(dsn, "postgres://") {
		dialect = pgdialect.New()
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	} else {
		dialect = sqlitedialect.New()
		sqliteDB, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
		if err != nil {
			color.Red("error opening connection for SQLite: %s", err)
			os.Exit(1101)
		}
		sqlDB = sqliteDB
	}

	sqlDB.SetMaxIdleConns(maxOpenConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	bunWrappedDB := bun.NewDB(sqlDB, dialect)
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to database: %s", err)
		os.Exit(1100)
	}

	store := registryStore{
		rawDB:  sqlDB,
		db:     bunWrappedDB,
		logger: logger,
	}

	if env == config.Local {
		bunWrappedDB.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithVerbose(true),
			bundebug.FromEnv("BUNDEBUG"),
		))
	}

	return &store
}
