package registry

import (
	"context"

	store_v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type registryStore struct {
	db     *bun.DB
	logger telemetry.Logger
}

func NewStore(
	db *bun.DB,
	logger telemetry.Logger,
) RegistryStore {
	store := registryStore{
		db:     db,
		logger: logger,
	}

	return &store
}

type RegistryBaseStore interface {
	SetLayer(ctx context.Context, txn *bun.Tx, l *types.ContainerImageLayer) error
	GetLayer(ctx context.Context, digest string) (*types.ContainerImageLayer, error)
	SetManifest(ctx context.Context, txn *bun.Tx, im *types.ImageManifest) error
	GetManifest(ctx context.Context, ref string) (*types.ImageManifest, error)
	GetManifestByReference(ctx context.Context, namespace string, ref string) (*types.ImageManifest, error)
	GetReferrers(ctx context.Context, digest string, artifactTypes ...string) (*types.ReferrerManifest, error)
}

type RegistryStore interface {
	// Postgres Transaction handlers
	store_v2.PgTxnHandler

	// The base registry store methods
	RegistryBaseStore

	GetContentHashById(ctx context.Context, uuid string) (string, error)
	GetImageTags(ctx context.Context, namespace string) ([]string, error)
	GetCatalog(ctx context.Context, namespace string, pageSize int, offset int) ([]string, error)
	GetCatalogDetail(
		ctx context.Context, namespace string, pageSize int, offset int, sortBy string,
	) ([]*types.ContainerImageRepository, error)
	GetRepoDetail(
		ctx context.Context,
		namespace string,
		pageSize int,
		offset int,
	) (*types.ContainerImageRepository, error)
	GetCatalogCount(ctx context.Context, ns string) (int64, error)
	GetImageNamespace(ctx context.Context, search string) ([]*types.ImageManifest, error)
	DeleteLayerByDigest(ctx context.Context, digest string) error
	GetPublicRepositories(ctx context.Context, pageSize int, offset int) ([]*types.ContainerImageRepository, int, error)
	GetUserRepositories(
		ctx context.Context,
		userID uuid.UUID,
		visibility types.RepositoryVisibility,
		pageSize int,
		offset int,
	) ([]*types.ContainerImageRepository, int, error)
	DeleteLayerByDigestWithTxn(ctx context.Context, txn *bun.Tx, digest string) error
	DeleteManifestOrTag(ctx context.Context, reference string) error
	DeleteManifestOrTagWithTxn(ctx context.Context, txn *bun.Tx, reference string) error
	SetContainerImageVisibility(ctx context.Context, imageId string, visibility types.RepositoryVisibility) error

	CreateRepository(ctx context.Context, repository *types.ContainerImageRepository) error
	GetRepositoryByID(ctx context.Context, ID uuid.UUID) (*types.ContainerImageRepository, error)
	GetRepositoryByNamespace(ctx context.Context, namespace string) (*types.ContainerImageRepository, error)
	RepositoryExists(ctx context.Context, name string) bool
	GetRepositoryByName(ctx context.Context, userId uuid.UUID, name string) (*types.ContainerImageRepository, error)
}
