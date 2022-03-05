package postgres

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/containerish/OpenRegistry/types"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

func (p *pg) GetLayer(ctx context.Context, digest string) (*types.LayerV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetLayer, digest)
	var layer types.LayerV2
	if err := row.Scan(
		&layer.UUID,
		&layer.Digest,
		&layer.BlobDigests,
		&layer.MediaType,
		&layer.SkynetLink,
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

func (p *pg) GetManifest(ctx context.Context, namespace string) (*types.ImageManifestV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetManifest, namespace)
	var im types.ImageManifestV2
	if err := row.Scan(
		&im.Uuid,
		&im.Namespace,
		&im.MediaType,
		&im.SchemaVersion,
	); err != nil {
		return nil, err
	}
	return &im, nil
}
func (p *pg) GetManifestByReference(ctx context.Context, namespace string, ref string) (*types.ConfigV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	query := queries.GetManifestByRef
	if strings.HasPrefix(ref, "sha256") {
		query = queries.GetManifestByDig
	}

	row := p.conn.QueryRow(childCtx, query, namespace, ref)
	var im types.ConfigV2
	if err := row.Scan(
		&im.UUID,
		&im.Namespace,
		&im.Reference,
		&im.Digest,
		&im.Skylink,
		&im.MediaType,
		&im.Layers,
		&im.Size,
	); err != nil {
		return nil, err
	}
	return &im, nil
}

func (p *pg) SetManifest(ctx context.Context, txn pgx.Tx, im *types.ImageManifestV2) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := txn.Exec(childCtx, queries.SetImageManifest, im.Uuid, im.Namespace, im.MediaType, im.SchemaVersion)
	return err
}

func (p *pg) GetBlob(ctx context.Context, digest string) ([]*types.Blob, error) {

	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	rows, err := p.conn.Query(childCtx, queries.GetBlob, digest)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blobList := make([]*types.Blob, 0)
	for i := 0; rows.Next(); i++ {
		var blob types.Blob
		if err := rows.Scan(
			&blob.UUID,
			&blob.Digest,
			&blob.Skylink,
			&blob.RangeStart,
			&blob.RangeEnd,
		); err != nil {
			return nil, err
		}

		blobList = append(blobList, &blob)
	}

	return blobList, nil
}

func (p *pg) SetBlob(ctx context.Context, txn pgx.Tx, b *types.Blob) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := txn.Exec(childCtx, queries.SetBlob, b.UUID, b.Digest, b.Skylink, b.RangeStart, b.RangeEnd)
	return err

}

func (p *pg) GetConfig(ctx context.Context, namespace string) ([]*types.ConfigV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	rows, err := p.conn.Query(childCtx, queries.GetConfig, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cfgList := make([]*types.ConfigV2, 0)

	for i := 0; rows.Next(); i++ {
		var cfg types.ConfigV2
		if err := rows.Scan(
			&cfg.UUID,
			&cfg.Namespace,
			&cfg.Reference,
			&cfg.Digest,
			&cfg.Skylink,
			&cfg.MediaType,
			&cfg.Layers,
			&cfg.Size,
		); err != nil {
			return nil, err
		}

		cfgList = append(cfgList, &cfg)
	}

	return cfgList, nil
}
func (p *pg) GetImageTags(ctx context.Context, namespace string) ([]string, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	rows, err := p.conn.Query(childCtx, queries.GetImageTags, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string

	for i := 0; rows.Next(); i++ {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func (p *pg) SetConfig(ctx context.Context, txn pgx.Tx, cfg types.ConfigV2) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := txn.Exec(childCtx,
		queries.SetConfig,
		cfg.UUID,
		cfg.Namespace,
		cfg.Reference,
		cfg.Digest,
		cfg.Skylink,
		cfg.MediaType,
		cfg.Layers,
		cfg.Size,
	); err != nil {
		return err
	}
	return nil
}

func (p *pg) GetCatalogCount(ctx context.Context) (int64, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var count int64
	row := p.conn.QueryRow(childCtx, queries.GetCatalogCount)

	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("ERR_SCAN_CATALOG_COUNT: %w", err)
	}
	return count, nil
}

func (p *pg) GetCatalog(ctx context.Context, ns string, pageSize, offset int64) ([]*types.ConfigV2, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var rows pgx.Rows
	var err error

	if pageSize != 0 {
		rows, err = p.conn.Query(childCtx, queries.GetCatalogWithPagination, pageSize, offset)
		if err != nil {
			err = fmt.Errorf("ERR_CATALOG_WITH_PAGINATION: %w", err)
		}
	} else {
		rows, err = p.conn.Query(childCtx, queries.GetCatalog)
		if err != nil {
			err = fmt.Errorf("ERR_CATALOG: %w", err)
		}
	}
	if ns != "" {
		rows, err = p.conn.Query(childCtx, queries.GetUserCatalogWithPagination, ns+"/%", pageSize, offset)
		if err != nil {
			err = fmt.Errorf("ERR_USER_CATALOG: %w", err)
		}
	}
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	cfgList := make([]*types.ConfigV2, 0)
	for i := 0; rows.Next(); i++ {
		var cfg types.ConfigV2
		if err := rows.Scan(
			&cfg.UUID,
			&cfg.Namespace,
			&cfg.Reference,
			&cfg.Digest,
		); err != nil {
			return nil, err
		}
		cfgList = append(cfgList, &cfg)
	}
	return cfgList, nil
}

func (p *pg) DeleteLayerV2(ctx context.Context, txn pgx.Tx, digest string) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := txn.Exec(childCtx, queries.DeleteLayer, digest); err != nil {
		return err
	}
	return nil
}

func (p *pg) DeleteBlobV2(ctx context.Context, txn pgx.Tx, digest string) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := txn.Exec(childCtx, queries.DeleteBlob, digest); err != nil {
		return err
	}
	return nil
}

func (p *pg) DeleteManifestOrTag(ctx context.Context, txn pgx.Tx, reference string) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	query := queries.DeleteManifestByRef
	if strings.HasPrefix(reference, "sha256") {
		query = queries.DeleteManifestByDig
	}
	if _, err := txn.Exec(childCtx, query, reference); err != nil {
		return err
	}
	return nil
}

func (p *pg) NewTxn(ctx context.Context) (pgx.Tx, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	return p.conn.Begin(childCtx)
}

func (p *pg) Abort(ctx context.Context, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	return txn.Rollback(childCtx)
}

func (p *pg) Commit(ctx context.Context, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	return txn.Commit(childCtx)
}

func (p *pg) Metadata(ctx echo.Context) error {
	rows, err := p.conn.Query(ctx.Request().Context(), "select uuid, namespace from image_manifest")
	if err != nil {
		return err
	}
	defer rows.Close()

	var imageManifestList []*types.ImageManifestV2
	for rows.Next() {
		var im types.ImageManifestV2
		if err := rows.Scan(&im.Uuid, &im.Namespace); err != nil {
			return err
		}

		imageManifestList = append(imageManifestList, &im)
	}

	return ctx.JSON(http.StatusOK, imageManifestList)
}

func (p *pg) GetImageNamespace(ctx context.Context, search string) ([]string, error) {
	childCtx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()
	rows, err := p.conn.Query(childCtx, queries.GetImageNamespace, "%"+search+"%")
	if err != nil {
		return nil, fmt.Errorf("ERR_QUERY_GET_IMAGE_NAMESPACE: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			return nil, fmt.Errorf("ERR_IMAGE_NAMESPACE_SCAN: %w", err)
		}
		result = append(result, ns)
	}
	return result, nil
}
