package postgres

import (
	"context"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/labstack/echo/v4"
)

type PersistentStore interface {
	UserStore
	RegistryStore
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	GetDigest(digest string) (*types.LayerRef, error)
	SetDigest(digest, skylink string) error
	DeleteDigest(digest string) error
	DeleteLayer(namespace, digest string) error
	Update(key, value []byte) error
	ListAll() ([]byte, error)
	ListWithPrefix(prefix []byte) ([]byte, error)
	Delete(key []byte) error
	GetSkynetURL(key string, ref string) (string, error)
	UpdateManifestRef(namespace, ref string) error
	ResolveManifestRef(namespace, ref string) (string, error)
	Metadata(ctx echo.Context) error
	LayerDigests(ctx echo.Context) error
	RegisterForBeta(ctx echo.Context) error
	GetAllEmail(ctx echo.Context) error
	Close() error
}

type AuthStore interface {
	StoreToken() error
}

type UserStore interface {
	AddUser(ctx context.Context, u *types.User) error
	GetUser(ctx context.Context, identifier string) (*types.User, error)
	UpdateUser(ctx context.Context, identifier string, u *types.User) error
	DeleteUser(ctx context.Context, identifier string) error
	IsActive(ctx context.Context, identifier string) bool
}

type RegistryStore interface {
	NewTxn(ctx context.Context) (pgx.Tx, error)
	Abort(ctx context.Context, txn pgx.Tx) error
	Commit(ctx context.Context, txn pgx.Tx) error
	SetLayer(ctx context.Context, txn pgx.Tx, l *types.LayerV2) error
	SetManifest(ctx context.Context, txn pgx.Tx, im *types.ImageManifestV2) error
	SetBlob(ctx context.Context, txn pgx.Tx, b *types.Blob) error
	SetConfig(ctx context.Context, txn pgx.Tx, cfg types.ConfigV2) error
	GetManifest(ctx context.Context, ref string) (*types.ImageManifestV2, error)
	GetManifestByReference(ctx context.Context, namespace string, ref string) (*types.ConfigV2, error)
	GetLayer(ctx context.Context, digest string) (*types.LayerV2, error)
	GetBlob(ctx context.Context, digest string) ([]*types.Blob, error)
	GetConfig(ctx context.Context, namespace string) ([]*types.ConfigV2, error)
	GetImageTags(ctx context.Context, namespace string) ([]string, error)
	GetCatalog(ctx context.Context, namespace string, pageSize int64, offset int64) ([]*types.ConfigV2, error)
	GetCatalogCount(ctx context.Context) (int64, error)
	DeleteLayerV2(ctx context.Context, txn pgx.Tx, digest string) error
	DeleteBlobV2(ctx context.Context, txn pgx.Tx, digest string) error
	DeleteManifestOrTag(ctx context.Context, txn pgx.Tx, reference string) error
}

type pg struct {
	conn *pgxpool.Pool
}

func New(cfg *config.Store) (PersistentStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	pgxCofig, err := pgxpool.ParseConfig(cfg.Endpoint())
	if err != nil {
		return nil, err
	}

	conn, err := pgxpool.ConnectConfig(ctx, pgxCofig)
	if err != nil {
		return nil, err
	}

	color.Green("connection to database successful")
	return &pg{conn: conn}, nil
}

func (p *pg) RegisterForBeta(ctx echo.Context) error       { return nil }
func (p *pg) Get(key []byte) ([]byte, error)               { return nil, nil }
func (p *pg) Set(key, value []byte) error                  { return nil }
func (p *pg) Update(key, value []byte) error               { return nil }
func (p *pg) Delete(key []byte) error                      { return nil }
func (p *pg) ListAll() ([]byte, error)                     { return nil, nil }
func (p *pg) ListWithPrefix(prefix []byte) ([]byte, error) { return nil, nil }
func (p *pg) GetAllEmail(ctx echo.Context) error           { return nil }

func (p *pg) GetSkynetURL(key string, ref string) (string, error)      { return "", nil }
func (p *pg) DeleteLayer(namespace, digest string) error               { return nil }
func (p *pg) LayerDigests(ctx echo.Context) error                      { return nil }
func (p *pg) UpdateManifestRef(namespace, ref string) error            { return nil }
func (p *pg) ResolveManifestRef(namespace, ref string) (string, error) { return "", nil }

func (p *pg) GetDigest(digest string) (*types.LayerRef, error) { return nil, nil }
func (p *pg) SetDigest(digest, skylink string) error           { return nil }
func (p *pg) DeleteDigest(digest string) error                 { return nil }
func (p *pg) Close() error                                     { return nil }
