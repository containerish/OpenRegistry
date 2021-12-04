package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/containerish/OpenRegistry/types"
	"github.com/jackc/pgx/v4"
)

func (p *pg) GetLayer(ctx context.Context, txn pgx.Tx, digest string) (*types.Layer, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	row := txn.QueryRow(childCtx, queries.GetLayer, digest)
	var layer types.Layer
	if err := row.Scan(
		&layer.MediaType,
		&layer.Digest,
		&layer.SkynetLink,
		&layer.UUID,
		&layer.Blobs,
		&layer.Size,
	); err != nil {
		return nil, err
	}

	return &layer, nil

}

func (p *pg) SetLayer(ctx context.Context, txn pgx.Tx, l *types.LayerV2) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := txn.Exec(childCtx, queries.SetLayer, l.MediaType, l.Digest, l.SkynetLink, l.UUID, l.BlobDigests, l.Size)
	return err
}

func (p *pg) GetManifest(ctx context.Context, txn pgx.Tx, ref string) (*types.ImageManifestV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	row := txn.QueryRow(childCtx, queries.GetManifest, ref)
	var im *types.ImageManifestV2
	if err := row.Scan(
		&im.Uuid,
		&im.Namespace,
		&im.MediaType,
		&im.SchemaVersion,
	); err != nil {
		return nil, err
	}
	return im, nil
}

func (p *pg) SetManifest(ctx context.Context, txn pgx.Tx, im *types.ImageManifestV2) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := txn.Exec(childCtx, queries.SetImageManifest, im.Uuid, im.Namespace, im.MediaType, im.SchemaVersion)
	return err
}

func (p *pg) SetBlob(ctx context.Context, txn pgx.Tx, b *types.Blob) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := txn.Exec(childCtx, queries.SetBlob, b.UUID, b.Digest, b.Skylink, b.RangeStart, b.RangeEnd)
	return err

}

func (p *pg) GetBlob(ctx context.Context, txn pgx.Tx, digest string) (*types.Blob, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	row := txn.QueryRow(childCtx, queries.GetBlob, digest)
	if row == nil {
		return nil, fmt.Errorf("error blob not found")
	}
	var blob types.Blob
	if err := row.Scan(
		&blob.UUID,
		&blob.Digest,
		&blob.Skylink,
		&blob.RangeStart,
		&blob.RangeEnd,
	); err != nil {
		return nil, err
	}

	return &blob, nil
}

func (p *pg) NewTxn(ctx context.Context) (pgx.Tx, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	return p.conn.Begin(childCtx)
}

func (p *pg) Abort(ctx context.Context, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	return txn.Rollback(childCtx)
}

func (p *pg) Commit(ctx context.Context, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	return txn.Commit(childCtx)
}
