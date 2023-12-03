package v1

import (
	"context"
	"database/sql"
	"os"

	"github.com/containerish/OpenRegistry/config"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/schema"
)

func New(cfg config.Store, env config.Environment) *bun.DB {
	var (
		dialect schema.Dialect
		sqlDB   *sql.DB
	)

	switch cfg.Kind {
	case config.StoreKindPostgres:
		dialect = pgdialect.New()
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.Endpoint())))
	case config.StoreKindSQLite:
		dialect = sqlitedialect.New()
		sqliteDB, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
		if err != nil {
			color.Red("error opening connection for SQLite: %s", err)
			os.Exit(1101)
		}
		sqlDB = sqliteDB
	default:
		color.Red("Invalid store kind: %s", cfg.Kind)
		os.Exit(1101)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxOpenConnections)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConnections)

	bunWrappedDB := bun.NewDB(sqlDB, dialect)
	if err := bunWrappedDB.Ping(); err != nil {
		color.Red("error connecting to database: %s", err)
		os.Exit(1100)
	}

	if env == config.Local || env == config.Staging {
		bunWrappedDB.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithVerbose(false),
			bundebug.FromEnv("BUNDEBUG"),
		))
	}

	return bunWrappedDB
}

type DBPinger struct {
	DB *bun.DB
}

func (p *DBPinger) Ping(ctx context.Context) error {
	return p.DB.PingContext(ctx)
}
