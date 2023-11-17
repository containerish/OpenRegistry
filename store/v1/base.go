package v1

import (
	"context"

	"github.com/uptrace/bun"
)

type PgTxnHandler interface {
	NewTxn(ctx context.Context) (*bun.Tx, error)
	Abort(ctx context.Context, txn *bun.Tx) error
	Commit(ctx context.Context, txn *bun.Tx) error
}

type PostgresPing interface {
	Ping(ctx context.Context) error
}
