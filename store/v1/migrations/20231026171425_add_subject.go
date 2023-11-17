package migrations

import (
	"context"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
)

func init() {
	up := func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			color.Green("Running up migration ✅")
			_, err := tx.
				NewAddColumn().
				Model(&types.ImageManifest{}).
				ColumnExpr("subject jsonb").
				IfNotExists().
				Exec(ctx)
			return err
		})
	}

	down := func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			color.Yellow("Running down migration ⚠️")

			_, err := tx.
				NewDropColumn().
				Model(&types.ImageManifest{}).
				ColumnExpr("subject").
				Exec(ctx)
			return err
		})
	}

	Migrations.MustRegister(up, down)
}
