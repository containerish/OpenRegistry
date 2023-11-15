package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/fatih/color"
	"github.com/google/uuid"
	oci_digest "github.com/opencontainers/go-digest"
	img_spec "github.com/opencontainers/image-spec/specs-go"
	img_spec_v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/feature"
)

func (s *registryStore) RepositoryExists(ctx context.Context, name string) bool {
	logEvent := s.logger.Debug().Str("method", "RepositoryExists").Str("name", name)

	repository := &types.ContainerImageRepository{}
	if err := s.db.NewSelect().Model(repository).Scan(ctx); err != nil {
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
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	logEvent.Bool("success", true).Send()
	return nil
}

func (s *registryStore) GetRepositoryByID(ctx context.Context, ID uuid.UUID) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetRepositoryByID").Str("id", ID.String())

	repository := &types.ContainerImageRepository{ID: ID}
	if err := s.db.NewSelect().Model(repository).WherePK().Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return repository, nil
}

func (s *registryStore) GetRepositoryByNamespace(
	ctx context.Context,
	namespace string,
) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetRepositoryByNamespace").Str("namespace", namespace)

	repository := &types.ContainerImageRepository{}
	err := s.
		db.
		NewSelect().
		Model(repository).
		Where("name = ?", strings.Split(namespace, "/")[1]).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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
		Str("method", "GetRepositoryByNamespace").
		Str("name", name).
		Str("user_id", userId.String())

	var repository types.ContainerImageRepository
	err := s.
		db.
		NewSelect().
		Model(&repository).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("name = ?", name)
		}).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("owner_id = ?", userId)
		}).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return &repository, nil
}

func (s *registryStore) DeleteLayerByDigestWithTxn(ctx context.Context, txn *bun.Tx, digest string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteLayerByDigestWithTxn").Str("digest", digest)

	_, err := txn.NewDelete().Model(&types.ContainerImageLayer{}).Where("digest = ?", digest).Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

func (s *registryStore) DeleteLayerByDigest(ctx context.Context, digest string) error {
	logEvent := s.logger.Debug().Str("method", "DeleteLayerByDigest").Str("digest", digest)

	_, err := s.db.NewDelete().Model(&types.ContainerImageLayer{}).Where("digest = ?", digest).Exec(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
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
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
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
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
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
	var catalog []types.ContainerImageRepository

	repositoryName := strings.Split(namespace, "/")[1]
	err := s.
		db.
		NewSelect().
		Model(&catalog).
		Relation("ImageManifests").
		Where("name = ? and visibility = ?", repositoryName, types.RepositoryVisibilityPublic).
		Scan(ctx)
	if err != nil {
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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
		return nil, 0, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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
		return nil, 0, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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

	stmnt := s.
		db.
		NewSelect().
		Model(&types.ImageManifest{}).
		Relation("Repository").
		Where("visibility = ?", types.RepositoryVisibilityPublic)

	if repositoryName != "" {
		stmnt.Where("name = ?", repositoryName)
	}

	count, err := stmnt.Count(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return 0, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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

	stmnt := s.
		db.
		NewSelect().
		Model(&repositoryList).
		Relation("ImageManifests").
		Limit(pageSize).
		Offset(offset).
		Where("visibility = ?", types.RepositoryVisibilityPublic)

	if repositoryName != "" {
		stmnt.Where("name = ?", repositoryName)
	}

	err := stmnt.Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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
		return "", v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return dfsLink, nil
}

// GetImageNamespace implements registry.RegistryStore.
func (s *registryStore) GetImageNamespace(ctx context.Context, search string) ([]*types.ImageManifest, error) {
	logEvent := s.logger.Debug().Str("method", "GetImageNamespace").Str("search_query", search)
	var manifests []*types.ImageManifest

	err := s.db.NewSelect().Model(&manifests).Where("substr(namespace, 1, 50) LIKE ?", bun.Ident(search)).Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return manifests, nil
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
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
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
	var layer types.ContainerImageLayer
	if err := s.db.NewSelect().Model(&layer).Where("digest = ?", digest).Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return &layer, nil
}

// GetManifest implements registry.RegistryStore.
func (s *registryStore) GetManifest(ctx context.Context, id string) (*types.ImageManifest, error) {
	logEvent := s.logger.Debug().Str("method", "GetManifest").Str("id", id)
	var manifest types.ImageManifest
	if err := s.db.NewSelect().Model(&manifest).Where("id = ?", id).Scan(ctx); err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return &manifest, nil
}

// GetManifestByReference implements registry.RegistryStore.
// reference can either be a tag or a digest
func (s *registryStore) GetManifestByReference(
	ctx context.Context,
	namespace string,
	ref string,
) (*types.ImageManifest, error) {
	logEvent := s.logger.Debug().Str("method", "GetManifestByReference").Str("whereClause", "reference")

	var manifest types.ImageManifest
	err := s.
		db.
		NewSelect().
		Model(&manifest).
		WhereOr("reference = ?", ref).
		WhereOr("digest = ?", ref).
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return &manifest, nil
}

// GetRepoDetail implements registry.RegistryStore.
func (s *registryStore) GetRepoDetail(
	ctx context.Context,
	namespace string,
	pageSize int,
	offset int,
) (*types.ContainerImageRepository, error) {
	logEvent := s.logger.Debug().Str("method", "GetRepoDetail")
	var repoDetail types.ContainerImageRepository

	repositoryName := strings.Split(namespace, "/")[1]
	err := s.
		db.
		NewSelect().
		Model(&repoDetail).
		Relation("ImageManifests").
		Where("name = ?", repositoryName).
		Limit(pageSize).
		Offset(offset).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	logEvent.Bool("success", true).Send()
	return &repoDetail, nil
}

// SetContainerImageVisibility implements registry.RegistryStore.
func (s *registryStore) SetContainerImageVisibility(
	ctx context.Context,
	imageId string,
	visibility types.RepositoryVisibility,
) error {
	logEvent := s.logger.Debug().Str("method", "SetContainerImageVisibility")

	_, err := s.
		db.
		NewUpdate().
		Model(&types.ContainerImageRepository{}).
		Set("visibility = ?", visibility).
		WherePK(imageId).
		Exec(ctx)

	if err != nil {
		logEvent.Err(err).Send()
		return v2.WrapDatabaseError(err, v2.DatabaseOperationUpdate)
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
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
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
			return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
		}

		logEvent.Bool("success", true).Send()
		return nil
	}

	if s.db.HasFeature(feature.InsertOnDuplicateKey) {
		_, err := txn.NewInsert().Model(im).Exec(ctx)
		if err != nil {
			logEvent.Err(err).Send()
			return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
		}

		logEvent.Bool("success", true).Send()
		return nil
	}

	return v2.WrapDatabaseError(
		fmt.Errorf("DB_ERR: InsertOnUpdate feature not available"),
		v2.DatabaseOperationWrite,
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
		// WhereOr("COALESCE(subject_digest, '') = '' AND digest = ?", digest).
		// WhereOr("subject_digest = ?", digest)

		WhereGroup(" OR ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("subject_digest = ?", digest)
			// return sq.WhereOr("COALESCE(subject_digest, '') = '' AND digest = ?", digest).
			// 	WhereOr("subject_digest = ?", digest)
		})

	if len(artifactTypes) > 0 {
		color.Yellow("ArtifactType: %s", artifactTypes)

		var mfl []*types.ImageManifest
		s.db.NewSelect().Model(&mfl).Scan(ctx)
		bz, _ := json.MarshalIndent(mfl, "", "\t")
		color.Green("Available manifests: \n%s", bz)
		// q.
		// 	WhereOr("artifact_type IN (?)", bun.In(artifactTypes)).
		// 	WhereOr("COALESCE(artifact_type, '') = '' AND config_media_type IN (?)", bun.In(artifactTypes))

		q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.WhereOr("artifact_type IN (?)", bun.In(artifactTypes)).
				WhereOr("COALESCE(artifact_type, '') = '' AND config_media_type IN (?)", bun.In(artifactTypes))
		})

		// WhereOr("artifact_type = '' AND config_artifact_type IN (?)", bun.In(artifactTypes))
	}

	color.Magenta("query: %s", q.String())

	q.
		Relation("Repository", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("*").Where("name = ?", repoName)
		}).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.ExcludeColumn("*").Where("username = ?", username)
		})

	if err := q.Scan(ctx); err != nil {
		return imgIndex, nil
	}

	// q := s.
	// 	db.
	// 	NewSelect().
	// 	Model(&manifestList)
	//
	// // Find OCI Distribution Spec < 1.1 manifests to provide backword compatibility
	// // Reference - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#enabling-the-referrers-api
	// q.WhereGroup(" OR ", func(sq *bun.SelectQuery) *bun.SelectQuery {
	// 	return sq.
	// 		// for all new manifests with subject (OCI v1.1+), we set subject digest and manifest artifactType values
	// 		Where("subject_digest = '' AND artifact_type = '' AND digest = ?", digest)
	// }).WhereGroup(" OR ", func(sq *bun.SelectQuery) *bun.SelectQuery {
	// 	return sq.Where("subject_digest = ?", digest)
	// })
	//
	// if len(artifactTypes) > 0 {
	// 	q.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
	// 		return sq.Where("artifact_type in (?) or artifact_type = ''", bun.In(artifactTypes))
	// 	})
	// }

	// color.Magenta("GetReferrers: oldManifestsQuery: %s", oldMfQ.String())
	// color.Magenta("GetReferrers: newManifestsQuery: %s", newMfQ.String())
	// q.Relation("Repository", func(sq *bun.SelectQuery) *bun.SelectQuery {
	// 	return sq.ExcludeColumn("*").Where("name = ?", repoName)
	// }).Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
	// 	return sq.Where("username = ?", username).ExcludeColumn("*")
	// })

	// eChan := make(chan error)
	// go func() {
	// 	wg.Wait()
	// 	close(eChan)
	// }()
	//
	// go func() {
	// 	defer wg.Done()
	// 	if err := oldMfQ.Scan(ctx); err != nil {
	// 		eChan <- err
	// 		return
	// 	}
	// }()
	//
	// go func() {
	// 	defer wg.Done()
	// 	if err := newMfQ.Scan(ctx); err != nil {
	// 		eChan <- err
	// 		return
	// 	}
	// }()
	//
	// mErr := &multierror.Error{}
	// for err := range eChan {
	// 	if err != nil {
	// 		mErr.Errors = append(mErr.Errors, err)
	// 	}
	// }
	// if mErr.ErrorOrNil() != nil {
	// 	return imgIndex, mErr
	// }

	// obz, _ := json.MarshalIndent(oldManifests, "", "\t")
	// nbz, _ := json.MarshalIndent(manifests, "", "\t")
	// color.Blue("OldManifests: \n%s\nNewManifests: \n%s", obz, nbz)

	// seenMap := map[string]struct{}{}
	// for _, m := range append(oldManifests, newManifests...) {
	//
	// 	mapKey := fmt.Sprintf("%s_%s", m.MediaType, m.Digest)
	//
	// 	if m.Subject != nil {
	// 		mapKey = fmt.Sprintf("%s_%s_%s", m.MediaType, m.Digest, m.Subject.MediaType)
	// 	} else if m.Config != nil {
	// 		mapKey = fmt.Sprintf("%s_%s", m.Config.MediaType, m.Config.Digest)
	// 	}
	// 	if _, ok := seenMap[mapKey]; ok {
	// 		continue
	// 	}
	// 	if m.Subject != nil {
	// 		seenMap[mapKey] = struct{}{}
	// 		subjectList = append(subjectList, m.Subject)
	// 		continue
	// 	}
	//
	// seenMap[mapKey] = struct{}{}
	// 	subjectList = append(subjectList, &types.OCIDescriptor{
	// 		MediaType: m.MediaType,
	// 		Digest:    oci_digest.FromString(m.Digest),
	// 		Size:      int64(m.Size),
	// 	})
	// }

	// combinedManifests := append(oldManifests, manifests...)
	var descriptors []img_spec_v1.Descriptor

	for _, m := range manifests {
		d, err := oci_digest.Parse(m.Digest)
		// skip any invalid digest, though there shouldn't be any invalid digests, ideally.
		if err != nil {
			continue
		}

		if !s.descriptorFound(descriptors, d.String()) {
			if m.Subject != nil {
				descriptors = append(descriptors, img_spec_v1.Descriptor{
					MediaType:    m.MediaType,
					Digest:       d,
					Size:         int64(m.Size),
					ArtifactType: m.ArtifactType,
				})
			} else {
				if m.ArtifactType == "" {
					m.ArtifactType = m.Config.MediaType
				}
				//
				//
				// artifactType := m.ArtifactType
				// if artifactType == "" {
				// 	artifactType = m.Config.ArtifactType
				// }

				descriptors = append(descriptors, img_spec_v1.Descriptor{
					MediaType:    m.MediaType,
					Digest:       d,
					Size:         int64(m.Size),
					ArtifactType: m.ArtifactType,
				})
			}

		}

		// switch m.MediaType {
		// case img_spec_v1.MediaTypeImageManifest:
		// case img_spec_v1.MediaTypeImageIndex:
		// 	descriptors = append(descriptors, img_spec_v1.Descriptor{
		// 		Annotations:  m.Subject.Annotations,
		// 		MediaType:    m.MediaType,
		// 		Digest:       oci_digest.FromString(m.Digest),
		// 		Size:         int64(m.Size),
		// 		ArtifactType: m.ArtifactType,
		// 	})
		// default:
		// 	color.Red("----------------------------- FOUND_INVALID_MEDIA_TYPE -------------------------------------")
		// }
	}

	imgIndex.Manifests = descriptors
	return imgIndex, nil
}

func (s *registryStore) descriptorFound(descriptors []img_spec_v1.Descriptor, digest string) bool {
	color.Yellow("total descriptors: %d", len(descriptors))
	for _, d := range descriptors {
		color.Yellow("d.digest=%s - digest=%s - matched: %v", d.Digest.String(), digest, d.Digest.String() == digest)
		if d.Digest.String() == digest {
			return true
		}
	}

	return false
}

func (s *registryStore) GetImageSizeByLayerIds(ctx context.Context, layerIDs []string) (uint64, error) {
	var size uint64
	err := s.
		db.
		NewSelect().
		Model(&types.ContainerImageLayer{}).
		ColumnExpr("sum(size)").
		Where("digest in (?)", bun.In(layerIDs)).
		Scan(ctx, &size)
	if err != nil {
		return 0, err
	}

	return size, nil
}
