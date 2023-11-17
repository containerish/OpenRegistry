package migrations

import (
	"context"
	"strings"

	"github.com/fatih/color"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// A collection of migrations.
// nolint
var Migrations = migrate.NewMigrations(migrate.WithMigrationsDirectory("store/v2/migrations"))

func PerformMigrations(ctx context.Context, db *bun.DB) {
	if err := Migrations.DiscoverCaller(); err != nil {
		panic(color.RedString("error discovering migration files: %s", err))
	}

	migrator := NewMigrator(db)
	if err := migrator.Init(ctx); err != nil {
		panic(color.RedString("error while migrations init: %s", err))
	}

	if err := migrator.Lock(ctx); err != nil {
		panic(color.RedString("error while locking the database for migrations: %s", err))
	}
	defer func() {
		_ = migrator.Unlock(ctx)
	}()

	group, err := migrator.Migrate(ctx)
	if err != nil && !strings.Contains(err.Error(), "there are no migrations") {
		panic(color.RedString("error while performing migrations: %s", err))
	}

	if group == nil || group.IsZero() {
		color.Yellow("Service=Bun Message=\"No new migratios\"")
		return
	}

	color.Green("Migrated: %s", group)
}

func NewMigrator(db *bun.DB) *migrate.Migrator {
	return migrate.NewMigrator(
		db,
		Migrations,
		migrate.WithLocksTableName("open_registry_migration_locks"),
		migrate.WithTableName("open_registry_migrations"),
		migrate.WithMarkAppliedOnSuccess(true),
	)
}
