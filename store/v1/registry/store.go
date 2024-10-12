package registry

import (
	"context"

	"github.com/google/uuid"
	img_spec_v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/uptrace/bun"

	store_v2 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
)

type registryStore struct {
	db     *bun.DB
	logger telemetry.Logger
}

func New(
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
	GetReferrers(
		ctx context.Context,
		ns string,
		digest string,
		artifactTypes []string,
	) (*img_spec_v1.Index, error)
}

type RegistryStore interface {
	// Postgres Transaction handlers
	store_v2.PgTxnHandler

	// The base registry store methods
	RegistryBaseStore

	GetImageSizeByLayerIds(ctx context.Context, layerIDs []string) (int64, error)
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
	GetImageNamespace(
		ctx context.Context,
		search string,
		visibility types.RepositoryVisibility,
		userId uuid.UUID,
	) ([]*types.ContainerImageRepository, error)
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
	SetContainerImageVisibility(ctx context.Context, imageId uuid.UUID, visibility types.RepositoryVisibility) error

	CreateRepository(ctx context.Context, repository *types.ContainerImageRepository) error
	GetRepositoryByID(ctx context.Context, ID uuid.UUID) (*types.ContainerImageRepository, error)
	GetRepositoryByNamespace(ctx context.Context, namespace string) (*types.ContainerImageRepository, error)
	RepositoryExists(ctx context.Context, namespace string) bool
	GetRepositoryByName(ctx context.Context, userId uuid.UUID, name string) (*types.ContainerImageRepository, error)
	IncrementRepositoryPullCounter(ctx context.Context, repoID uuid.UUID) error
	AddRepositoryToFavorites(ctx context.Context, repoID uuid.UUID, userID uuid.UUID) error
	RemoveRepositoryFromFavorites(ctx context.Context, repoID uuid.UUID, userID uuid.UUID) error
	GetLayersLinksForManifest(ctx context.Context, manifestDigest string) ([]*types.ContainerImageLayer, error)
	ListFavoriteRepositories(ctx context.Context, userID uuid.UUID) ([]*types.ContainerImageRepository, error)
}
