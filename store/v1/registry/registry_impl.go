package registry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	oci_digest "github.com/opencontainers/go-digest"
	img_spec "github.com/opencontainers/image-spec/specs-go"
	img_spec_v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/feature"

	v1 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (s *registryStore) RepositoryExists(ctx context.Context, namespace string) bool {
	logEvent := s.logger.Debug().Str("method", "RepositoryExists").Str("name", namespace)
	nsParts := strings.Split(namespace, "/")
	username, repoName := nsParts[0], nsParts[1]

	repository := &types.ContainerImageRepository{}
	err := s.
		db.
		NewSelect().
		Model(repository).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Column("username")
		}).
		Where("username = ?", username).
		Where("name = ?", repoName).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return false
	}

	logEvent.Bool("success", true).Send()
	return true
}

func (s *registryStore) CreateRepository(ctx context.Context, repository *types.ContainerImageRepository) error {
	logEvent := s.logger.Debug().Str("method", "CreateRepository").Str("name", repository.Name)

	if len(repository.ID) == 0 {
		repository.ID = uuid.New()
	}

	if _, err := s.db.NewInsert().Model(repository).Exec(ctx); err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

func (s *registryStore) GetRepositoryByID(ctx context.Context, ID uuid.UUID) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetRepositoryByID").Str("id", ID.String())

	repository := &types.ContainerImageRepository{ID: ID}
	if err := s.db.NewSelect().Model(repository).WherePK().Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repository, nil
}

func (s *registryStore) GetRepositoryByNamespace(
	ctx context.Context,
	namespace string,
) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("namespace", namespace)
	nsParts := strings.Split(namespace, "/")
	if len(nsParts) != 2 {
		return nil, fmt.Errorf("GetRepositoryByNamespace: invalid namespace format")
	}

	username, repoName := nsParts[0], nsParts[1]
	repository := &types.ContainerImageRepository{}
	err := s.
		db.
		NewSelect().
		Model(repository).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Column("username")
		}).
		Where("name = ?", repoName).Where("username = ?", username).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repository, nil
}

func (s *registryStore) GetRepositoryByName(
	ctx context.Context,
	userId uuid.UUID,
	name string,
) (*types.ContainerImageRepository, error) {
	logEvent := s.
		logger.
		Debug().
		Str("method", "GetRepositoryByName").
		Str("name", name).
		Str("user_id", userId.String())

	repository := &types.ContainerImageRepository{}
	err := s.
		db.
		NewSelect().
		Model(repository).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("name = ?", name)
		}).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("owner_id = ?", userId)
		}).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repository, nil
}

func (s *registryStore) DeleteLayerByDigestWithTxn(ctx context.Context, txn *bun.Tx, digest string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteLayerByDigestWithTxn").Str("digest", digest)

	_, err := txn.NewDelete().Model(&types.ContainerImageLayer{}).Where("digest = ?", digest).Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	return nil
}

func (s *registryStore) DeleteLayerByDigest(ctx context.Context, digest string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteLayerByDigest").Str("digest", digest)

	_, err := s.db.NewDelete().Model(&types.ContainerImageLayer{}).Where("digest = ?", digest).Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

// DeleteManifestOrTag implements registry.RegistryStore.
func (s *registryStore) DeleteManifestOrTag(ctx context.Context, reference string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteManifestOrTag").Str("reference", reference)

	_, err := s.
		db.
		NewDelete().
		Model(&types.ImageManifest{}).
		WhereOr("reference = ?", reference).
		WhereOr("digest = ? ", reference).
		Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

func (s *registryStore) DeleteManifestOrTagWithTxn(ctx context.Context, txn *bun.Tx, reference string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteManifestOrTagWithTxn").Str("reference", reference)

	_, err := txn.
		NewDelete().
		Model(&types.ImageManifest{}).
		WhereOr("reference = ?", reference).
		WhereOr("digest = ? ", reference).
		Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

// GetCatalog implements registry.RegistryStore.
func (s *registryStore) GetCatalog(
	ctx context.Context,
	namespace string,
	pageSize int,
	offset int,
) ([]string, error) {
	var catalog []*types.ContainerImageRepository

	repositoryName := strings.Split(namespace, "/")[1]
	err := s.
		db.
		NewSelect().
		Model(&catalog).
		Relation("ImageManifests").
		Where("name = ? and visibility = ?", repositoryName, types.RepositoryVisibilityPublic).
		Scan(ctx)
	if err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	namespaceList := make([]string, len(catalog))
	for i, m := range catalog {
		namespaceList[i] = m.ID.String()
	}

	return namespaceList, nil
}

func (s *registryStore) GetPublicRepositories(
	ctx context.Context,
	pageSize int,
	offset int,
) ([]*types.ContainerImageRepository, int, error) {
	repositories := []*types.ContainerImageRepository{}

	total, err := s.
		db.
		NewSelect().
		Model(&repositories).
		Where("visibility = ?", types.RepositoryVisibilityPublic).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return repositories, total, nil
}

func (s *registryStore) GetUserRepositories(
	ctx context.Context,
	userID uuid.UUID,
	visibility types.RepositoryVisibility,
	pageSize int,
	offset int,
) ([]*types.ContainerImageRepository, int, error) {
	repositories := []*types.ContainerImageRepository{}

	total, err := s.
		db.
		NewSelect().
		Model(&repositories).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("password").ExcludeColumn("github_connected").ExcludeColumn("webauthn_connected")
		}).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			if visibility != "" {
				return q.Where("visibility = ?", visibility)
			}

			return q.
				Where("visibility = ?", types.RepositoryVisibilityPublic).
				WhereOr("visibility = ?", types.RepositoryVisibilityPrivate)
		}).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("owner_id = ?", userID)
		}).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return repositories, total, nil
}

// GetCatalogCount implements registry.RegistryStore.
func (s *registryStore) GetCatalogCount(ctx context.Context, namespace string) (int64, error) {
	logEvent := s.logger.Debug().Str("method", "GetCatalogCount").Str("namespace", namespace)
	parts := strings.Split(namespace, "/")
	repositoryName := ""
	if len(parts) == 2 {
		repositoryName = parts[1]
	}

	q := s.
		db.
		NewSelect().
		Model(&types.ImageManifest{}).
		Relation("Repository").
		Where("visibility = ?", types.RepositoryVisibilityPublic)

	if repositoryName != "" {
		q.Where("name = ?", repositoryName)
	}

	count, err := q.Count(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return 0, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return int64(count), nil
}

// GetCatalogDetail implements registry.RegistryStore.
func (s *registryStore) GetCatalogDetail(
	ctx context.Context,
	namespace string,
	pageSize int,
	offset int,
	sortBy string,
) ([]*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetCatalogDetail").Str("namespace", namespace)
	var repositoryList []*types.ContainerImageRepository
	parts := strings.Split(namespace, "/")
	repositoryName := ""
	if len(parts) == 2 {
		repositoryName = parts[1]
	}

	q := s.
		db.
		NewSelect().
		Model(&repositoryList).
		Relation("User").
		Relation("ImageManifests").
		Limit(pageSize).
		Offset(offset).
		Where("visibility = ?", types.RepositoryVisibilityPublic)

	if repositoryName != "" {
		q.Where("name = ?", repositoryName)
	}

	err := q.Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repositoryList, nil
}

// GetContentHashById implements registry.RegistryStore.
func (s *registryStore) GetContentHashById(ctx context.Context, uuid string) (string, error) {
	logEvent := s.logger.Debug().Str("method", "GetContentHashById").Str("id", uuid)
	var dfsLink string
	err := s.db.NewSelect().Model(&types.ContainerImageLayer{}).Column("dfs_link").WherePK(uuid).Scan(ctx, &dfsLink)
	if err != nil {
		logEvent.Err(err).Send()
		return "", v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return dfsLink, nil
}

func (s *registryStore) excludeUserSensitiveFieldsOnJoin(sq *bun.SelectQuery) *bun.SelectQuery {
	return sq.ExcludeColumn("password").ExcludeColumn("created_at").ExcludeColumn("updated_at")
}

// GetImageNamespace implements registry.RegistryStore.
func (s *registryStore) GetImageNamespace(
	ctx context.Context,
	search string,
	visibility types.RepositoryVisibility,
	userID uuid.UUID,
) ([]*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetImageNamespace").Str("search_query", search)
	var repos []*types.ContainerImageRepository

	q := s.
		db.
		NewSelect().
		Model(&repos).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return s.excludeUserSensitiveFieldsOnJoin(sq)
		}).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.
				WhereOr("visibility = ?", types.RepositoryVisibilityPublic.String()).
				WhereOr("owner_id = ?", userID)
		}).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.WhereOr("substr(username, 1, 50) ILIKE ?", search).
				WhereOr("substr(name, 1, 50) ILIKE ?", search)
		})

	if visibility == types.RepositoryVisibilityPublic {
		q.Where("visibility = ? ", visibility.String())
	}

	if err := q.Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repos, nil
}

// GetImageTags implements registry.RegistryStore.
func (s *registryStore) GetImageTags(ctx context.Context, namespace string) ([]string, error) {
	logEvent := s.logger.Debug().Str("methid", "GetImageTags").Str("namespace", namespace)
	var manifests []*types.ImageManifest

	err := s.
		db.
		NewSelect().
		Model(&manifests).
		Relation("Repository").
		Column("reference").
		Where("name = ?", strings.Split(namespace, "/")[1]).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()

	tags := make([]string, 0)
	for _, manifest := range manifests {
		tags = append(tags, manifest.Reference)
	}

	return tags, nil
}

// GetLayer implements registry.RegistryStore.
func (s *registryStore) GetLayer(ctx context.Context, digest string) (*types.ContainerImageLayer, error) {
	logEvent := s.logger.Debug().Str("method", "GetLayer").Str("digest", digest)
	layer := &types.ContainerImageLayer{}
	if err := s.db.NewSelect().Model(layer).Where("digest = ?", digest).Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return layer, nil
}

// GetManifest implements registry.RegistryStore.
func (s *registryStore) GetManifest(ctx context.Context, id string) (*types.ImageManifest, error) {
	logEvent := s.logger.Debug().Str("method", "GetManifest").Str("id", id)
	manifest := &types.ImageManifest{}
	if err := s.db.NewSelect().Model(manifest).Where("id = ?", id).Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return manifest, nil
}

// GetManifestByReference implements registry.RegistryStore.
// reference can either be a tag or a digest
func (s *registryStore) GetManifestByReference(
	ctx context.Context,
	namespace string,
	ref string,
) (*types.ImageManifest, error) {
	logEvent := s.logger.Debug().Str("method", "GetManifestByReference").Str("whereClause", "reference")

	nsParts := strings.Split(namespace, "/")
	username, repoName := nsParts[0], nsParts[1]

	manifest := &types.ImageManifest{}
	q := s.
		db.
		NewSelect().
		Model(manifest).
		Relation("Repository", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Column("name")
		}).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Column("username")
		}).
		Where("username = ?", username).
		Where("name = ?", repoName)

	// check if ref is a digest
	digest, err := oci_digest.Parse(ref)
	if err == nil {
		q.Where("digest = ?", digest.String())
	} else {
		q.Where("reference = ?", ref)
	}

	if err := q.Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return manifest, nil
}

// GetRepoDetail implements registry.RegistryStore.
func (s *registryStore) GetRepoDetail(
	ctx context.Context,
	namespace string,
	pageSize int,
	offset int,
) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetRepoDetail")
	repoDetail := &types.ContainerImageRepository{}

	repositoryName := strings.Split(namespace, "/")[1]
	err := s.
		db.
		NewSelect().
		Model(repoDetail).
		Relation("ImageManifests").
		Where("name = ?", repositoryName).
		Limit(pageSize).
		Offset(offset).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repoDetail, nil
}

// SetContainerImageVisibility implements registry.RegistryStore.
func (s *registryStore) SetContainerImageVisibility(
	ctx context.Context,
	imageId uuid.UUID,
	visibility types.RepositoryVisibility,
) error {
	logEvent := s.logger.Debug().Str("method", "SetContainerImageVisibility")

	_, err := s.
		db.
		NewUpdate().
		Model(&types.ContainerImageRepository{ID: imageId}).
		Set("visibility = ?", visibility.String()).
		WherePK().
		Where("name != ?", types.SystemUsernameIPFS). // IPFS repositories cannot be set to private since they are P2P
		Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

// SetLayer implements registry.RegistryStore.
func (s *registryStore) SetLayer(ctx context.Context, txn *bun.Tx, l *types.ContainerImageLayer) error {
	logEvent := s.logger.Debug().Str("method", "SetLayer")
	_, err := txn.NewInsert().Model(l).On("conflict (digest) do update").Set("updated_at = ?", time.Now()).Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

// SetManifest implements registry.RegistryStore.
func (s *registryStore) SetManifest(ctx context.Context, txn *bun.Tx, im *types.ImageManifest) error {
	logEvent := s.logger.Debug().Str("method", "SetManifest")
	if im.ID.String() == "" {
		im.ID = uuid.New()
	}

	if s.db.HasFeature(feature.InsertOnConflict) {
		_, err := txn.
			NewInsert().
			Model(im).
			On("conflict (reference,repository_id) do update").
			Set("updated_at = ?", time.Now()).
			Exec(ctx)
		if err != nil {
			logEvent.Err(err).Send()
			return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
		}

		logEvent.Bool("success", true).Send()
		return nil
	}

	if s.db.HasFeature(feature.InsertOnDuplicateKey) {
		_, err := txn.NewInsert().Model(im).Exec(ctx)
		if err != nil {
			logEvent.Err(err).Send()
			return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
		}

		logEvent.Bool("success", true).Send()
		return nil
	}

	return v1.WrapDatabaseError(
		fmt.Errorf("DB_ERR: InsertOnUpdate feature not available"),
		v1.DatabaseOperationWrite,
	)
}

// NewTxn implements registry.RegistryStore.
func (s *registryStore) NewTxn(ctx context.Context) (*bun.Tx, error) {
	txn, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	return &txn, err
}

// Abort implements registry.RegistryStore.
func (s *registryStore) Abort(ctx context.Context, txn *bun.Tx) error {
	return txn.Rollback()
}

// Commit implements registry.RegistryStore.
func (s *registryStore) Commit(ctx context.Context, txn *bun.Tx) error {
	return txn.Commit()
}

func (s *registryStore) GetReferrers(
	ctx context.Context,
	namespace string,
	digest string,
	artifactTypes []string,
) (*img_spec_v1.Index, error) {
	var manifests []*types.ImageManifest
	// we return an empty list on error
	imgIndex := &img_spec_v1.Index{
		Versioned: img_spec.Versioned{
			SchemaVersion: 2,
		},
		MediaType: img_spec_v1.MediaTypeImageIndex,
	}
	nsParts := strings.Split(namespace, "/")
	if len(nsParts) != 2 {
		return imgIndex, fmt.Errorf("GetReferrers: invalid namespace format")
	}

	username, repoName := nsParts[0], nsParts[1]

	q := s.
		db.
		NewSelect().
		Model(&manifests).
		WhereGroup(" OR ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("subject_digest = ?", digest)
		})

	if len(artifactTypes) > 0 {
		q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.WhereOr("artifact_type IN (?)", bun.In(artifactTypes)).
				WhereOr("COALESCE(artifact_type, '') = '' AND config_media_type IN (?)", bun.In(artifactTypes))
		})
	}

	q.
		Relation("Repository", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("*").Where("name = ?", repoName)
		}).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("*").Where("username = ?", username)
		})

	if err := q.Scan(ctx); err != nil {
		return imgIndex, err
	}

	var descriptors []img_spec_v1.Descriptor

	for _, m := range manifests {
		d, err := oci_digest.Parse(m.Digest)
		// skip any invalid digest, though there shouldn't be any invalid digests, ideally.
		if err != nil {
			continue
		}

		if m.Subject != nil {
			descriptor := img_spec_v1.Descriptor{
				MediaType:    m.MediaType,
				Digest:       d,
				Size:         m.Size,
				ArtifactType: m.ArtifactType,
				Annotations:  m.Annotations,
			}

			descriptors = append(descriptors, descriptor)
		} else {
			if m.ArtifactType == "" {
				m.ArtifactType = m.Config.MediaType
			}
			descriptor := img_spec_v1.Descriptor{
				MediaType:    m.MediaType,
				Digest:       d,
				Size:         m.Size,
				ArtifactType: m.ArtifactType,
				Annotations:  m.Annotations,
			}

			descriptors = append(descriptors, descriptor)
		}

	}

	imgIndex.Manifests = descriptors
	return imgIndex, nil
}

func (s *registryStore) GetImageSizeByLayerIds(ctx context.Context, layerIDs []string) (int64, error) {
	var size int64
	err := s.
		db.
		NewSelect().
		Model(&types.ContainerImageLayer{}).
		ColumnExpr("sum(size)").
		Where("digest in (?)", bun.In(layerIDs)).
		Scan(ctx, &size)
	if err != nil {
		return 0, v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return size, nil
}

func (s *registryStore) IncrementRepositoryPullCounter(ctx context.Context, repoID uuid.UUID) error {
	repo := &types.ContainerImageRepository{
		ID: repoID,
	}

	_, err := s.db.NewUpdate().Model(repo).WherePK().Set("pull_count = pull_count + 1").Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return nil
}

func (s *registryStore) AddRepositoryToFavorites(ctx context.Context, repoID uuid.UUID, userID uuid.UUID) error {
	user := &types.User{}

	q := s.
		db.
		NewUpdate().
		Model(user).
		Set("favorite_repositories = array_append(favorite_repositories, ?)", repoID).
		Where("id = ?", userID).
		Where("NOT (? = ANY(favorite_repositories))", repoID)

	result, err := q.Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	if rowsAffected == 1 {
		repo := &types.ContainerImageRepository{
			ID: repoID,
		}

		_, err = s.db.NewUpdate().Model(repo).WherePK().Set("favorite_count = favorite_count + 1").Exec(ctx)
		if err != nil {
			return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
		}

		return nil
	}

	return v1.WrapDatabaseError(fmt.Errorf("repository is already in favorites list"), v1.DatabaseOperationUpdate)
}

func (s *registryStore) RemoveRepositoryFromFavorites(ctx context.Context, repoID uuid.UUID, userID uuid.UUID) error {
	user := &types.User{}
	q := s.
		db.
		NewUpdate().
		Model(user).
		Set("favorite_repositories = array_remove(favorite_repositories, ?)", repoID).
		Where("id = ?", userID).
		Where("? = ANY(favorite_repositories)", repoID)

	result, err := q.Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 1 {
		repo := &types.ContainerImageRepository{
			ID: repoID,
		}

		_, err = s.db.NewUpdate().Model(repo).WherePK().Set("favorite_count = favorite_count - 1").Exec(ctx)
		if err != nil {
			return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
		}
		return nil
	}

	return v1.WrapDatabaseError(
		fmt.Errorf("repository is not in favorites list"),
		v1.DatabaseOperationUpdate,
	)
}

func (s *registryStore) GetLayersLinksForManifest(
	ctx context.Context,
	manifestDigest string,
) ([]*types.ContainerImageLayer, error) {
	logEvent := s.logger.Debug().Str("method", "GetLayersLinksForManifest").Str("digest", manifestDigest)

	manifest := &types.ImageManifest{}
	err := s.db.NewSelect().Model(manifest).Where("digest = ?", manifestDigest).Scan(ctx)
	if err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	layerDigests := make([]string, len(manifest.Layers))
	for i, l := range manifest.Layers {
		layerDigests[i] = l.Digest.String()
	}

	var layers []*types.ContainerImageLayer
	if err = s.
		db.
		NewSelect().
		Model(&layers).
		Where("digest in (?)", bun.In(layerDigests)).
		Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return layers, nil
}
func (s *registryStore) ListFavoriteRepositories(
	ctx context.Context,
	userID uuid.UUID,
) ([]*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "ListFavoriteRepositories")

	repositories := []*types.ContainerImageRepository{}
	user := &types.User{ID: userID}
	err := s.
		db.
		NewSelect().
		Model(user).
		WherePK().
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	if len(user.FavoriteRepositories) == 0 {
		return repositories, nil
	}

	q := s.
		db.
		NewSelect().
		Model(&repositories).
		Where(`"r"."id" in (?)`, bun.In(user.FavoriteRepositories)).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("password").ExcludeColumn("github_connected").ExcludeColumn("webauthn_connected")
		})

	if err := q.Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return repositories, nil
}
