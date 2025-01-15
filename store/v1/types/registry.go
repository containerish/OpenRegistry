package types

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	img_spec_v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/uptrace/bun"
)

const (
	SystemUsernameIPFS   = "ipfs"
	HttpEndpointErrorKey = "HTTP_ERROR"
	HandlerStartTime     = "HANDLER_START_TIME"
)

func (v RepositoryVisibility) String() string {
	switch v {
	case RepositoryVisibilityPrivate:
		return "Private"
	case RepositoryVisibilityPublic:
		return "Public"
	default:
		return "Private"
	}
}

type (
	ContainerImageVisibilityChangeRequest struct {
		Visibility   RepositoryVisibility `json:"visibility_mode"`
		RepositoryID uuid.UUID            `json:"repository_id"`
	}

	ImageManifest struct {
		bun.BaseModel `bun:"table:image_manifests,alias:m" json:"-"`

		CreatedAt     time.Time                 `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
		UpdatedAt     time.Time                 `bun:"updated_at,nullzero" json:"updatedAt"`
		Repository    *ContainerImageRepository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
		User          *User                     `bun:"rel:belongs-to,join:owner_id=id" json:"-"`
		Subject       *img_spec_v1.Descriptor   `bun:"embed:subject_" json:"subject,omitempty"`
		Config        *img_spec_v1.Descriptor   `bun:"embed:config_" json:"config"`
		Annotations   map[string]string         `bun:"annotations,type:jsonb" json:"annotations,omitempty"`
		Digest        string                    `bun:"digest,notnull" json:"digest"`
		MediaType     string                    `bun:"media_type,notnull" json:"mediaType"`
		ArtifactType  string                    `bun:"artifact_type" json:"artifactType,omitempty"`
		Reference     string                    `bun:"reference,notnull" json:"reference"`
		Layers        ImageManifestLayers       `bun:"layers,type:jsonb" json:"layers"`
		SchemaVersion int                       `bun:"schema_version,notnull" json:"schemaVersion"`
		Size          int64                     `bun:"size,notnull" json:"size"`
		RepositoryID  uuid.UUID                 `bun:"repository_id,type:uuid" json:"repositoryId"`
		ID            uuid.UUID                 `bun:"id,pk,type:uuid" json:"id"`
		OwnerID       uuid.UUID                 `bun:"owner_id,type:uuid" json:"ownerId"`
	}

	Platform struct {
		Architecture string   `json:"architecture,omitempty"`
		Variant      string   `json:"variant,omitempty"`
		OS           string   `json:"os,omitempty"`
		OSVersion    string   `json:"os.version,omitempty"`
		OSFeatures   []string `json:"os.features,omitempty"`
	}

	ImageManifestPlatformOS struct {
		Name     string   `json:"name"`
		Version  string   `json:"version"`
		Features []string `json:"features"`
	}

	ImageManifestLayers []*img_spec_v1.Descriptor

	ContainerImageLayer struct {
		bun.BaseModel `bun:"table:layers,alias:l" json:"-"`

		CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"createdA"`
		UpdatedAt time.Time `bun:"updated_at,nullzero" json:"updatedAt"`
		ID        string    `bun:"id,pk,type:uuid" json:"id"`
		Digest    string    `bun:"digest,notnull,unique" json:"digest"`
		MediaType string    `bun:"media_type,notnull" json:"mediaType"`
		DFSLink   string    `bun:"dfs_link" json:"dfsLink"`
		Size      int64     `bun:"size,default:0" json:"size"`
	}

	ContainerImageRepository struct {
		bun.BaseModel `bun:"table:repositories,alias:r" json:"-"`

		CreatedAt      time.Time            `bun:"created_at" json:"created_at"`
		UpdatedAt      time.Time            `bun:"updated_at" json:"updated_at"`
		MetaTags       map[string]any       `bun:"meta_tags" json:"meta_tags"`
		User           *User                `bun:"rel:belongs-to,join:owner_id=id" json:"user"`
		Project        *RepositoryBuild     `bun:"rel:has-one,join:id=repository_id" json:"-"`
		Name           string               `bun:"name,notnull" json:"name"`
		Visibility     RepositoryVisibility `bun:"visibility,notnull" json:"visibility"`
		Description    string               `bun:"description" json:"description"`
		ImageManifests []*ImageManifest     `bun:"rel:has-many,join:id=repository_id" json:"image_manifests,omitempty"`
		Builds         []*RepositoryBuild   `bun:"rel:has-many,join:id=repository_id" json:"-"`
		PullCount      uint64               `bun:"pull_count" json:"pull_count"`
		FavoriteCount  uint64               `bun:"favorite_count" json:"favorite_count"`
		ID             uuid.UUID            `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
		OwnerID        uuid.UUID            `bun:"owner_id,type:uuid" json:"owner_id"`
	}

	RepositoryVisibility string
)

var _ driver.Valuer = (*ImageManifestLayers)(nil)

var _ sql.Scanner = (*ImageManifestLayers)(nil)

func (l ImageManifestLayers) Value() (driver.Value, error) {
	if len(l) == 0 {
		return nil, nil
	}

	bz, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(bz), nil
}

func (l *ImageManifestLayers) Scan(input any) error {
	if input == nil {
		return nil
	}

	bz, ok := input.([]byte)
	if !ok {
		return fmt.Errorf("Scan: expected []byte, got %T", input)
	}

	if err := json.Unmarshal(bz, l); err != nil {
		return fmt.Errorf("ERR_UNMARSHAL_MANIFEST: %w", err)
	}

	return nil
}

// ToOCISubject is a convenience method that returns a new copy of teh ImageManifest type, which only has the fields
// required by the OCI Image Manifest type
func (m *ImageManifest) ToOCISubject() []byte {
	if m == nil {
		return nil
	}

	manifest := map[string]any{
		"config":        m.Config,
		"mediaType":     m.MediaType,
		"layers":        m.Layers,
		"schemaVersion": m.SchemaVersion,
	}

	if m.ArtifactType != "" {
		manifest["artifactType"] = m.ArtifactType
	}

	if m.Subject != nil {
		manifest["subject"] = m.Subject
	}

	bz, _ := json.MarshalIndent(manifest, "", "\t")
	return bz
}

const (
	RepositoryVisibilityPublic  RepositoryVisibility = "Public"
	RepositoryVisibilityPrivate RepositoryVisibility = "Private"
)

var _ bun.BeforeAppendModelHook = (*ImageManifest)(nil)

var _ bun.BeforeAppendModelHook = (*ContainerImageLayer)(nil)

var _ bun.BeforeAppendModelHook = (*ContainerImageRepository)(nil)

func (imf *ImageManifest) String() string {
	return fmt.Sprintf("%#v", imf)
}

func (l *ContainerImageLayer) String() string {
	return fmt.Sprintf("%#v", l)
}

func (cir *ContainerImageRepository) String() string {
	return fmt.Sprintf("%#v", cir)
}

func (imf *ImageManifest) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		imf.CreatedAt = time.Now()
	case *bun.UpdateQuery:
		imf.UpdatedAt = time.Now()
	}

	return nil
}

func (l *ContainerImageLayer) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		l.CreatedAt = time.Now()
	case *bun.UpdateQuery:
		l.UpdatedAt = time.Now()
	}

	return nil
}

func (cir *ContainerImageRepository) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		cir.CreatedAt = time.Now()
	case *bun.UpdateQuery:
		cir.UpdatedAt = time.Now()
	}

	return nil
}

var _ bun.AfterCreateTableHook = (*ImageManifest)(nil)

var _ bun.AfterCreateTableHook = (*ContainerImageLayer)(nil)

var _ bun.AfterCreateTableHook = (*ContainerImageRepository)(nil)

func (cir *ContainerImageRepository) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.DB().NewCreateIndex().IfNotExists().Model(cir).Index("name_idx").Column("name").Exec(ctx)
	if err != nil {
		return err
	}

	color.Yellow(`Create index in table "repositories" on column "name" succeeded ✔︎`)
	return nil
}

func (l *ContainerImageLayer) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.DB().NewCreateIndex().IfNotExists().Model(l).Index("digest_idx").Column("digest").Exec(ctx)
	if err != nil {
		return err
	}

	color.Yellow(`Create index in table "layers" on column "digest" succeeded ✔︎`)
	return nil
}

func (imf *ImageManifest) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.DB().NewCreateIndex().IfNotExists().Model(imf).Index("digest_idx").Column("digest").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Create index in table "image_manifests" on column "digest" succeeded ✔︎`)
	_, err = query.DB().NewCreateIndex().IfNotExists().Model(imf).Index("reference_idx").Column("reference").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Create index in table "image_manifests" on column "reference" succeeded ✔︎`)
	return nil
}

var _ bun.AfterDropTableHook = (*ImageManifest)(nil)

var _ bun.AfterDropTableHook = (*ContainerImageLayer)(nil)

var _ bun.AfterDropTableHook = (*ContainerImageRepository)(nil)

func (imf *ImageManifest) AfterDropTable(ctx context.Context, query *bun.DropTableQuery) error {
	_, err := query.DB().NewDropIndex().IfExists().Model(imf).Index("digest_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "image_manifests" on column "digest" succeeded ✔︎`)
	_, err = query.DB().NewDropIndex().IfExists().Model(imf).Index("reference_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "image_manifests" on column "reference" succeeded ✔︎`)
	return nil
}

func (cir *ContainerImageRepository) AfterDropTable(ctx context.Context, query *bun.DropTableQuery) error {
	_, err := query.DB().NewDropIndex().IfExists().Model(cir).Index("name_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "repositories" on column "name" succeeded ✔︎`)
	return nil
}

func (l *ContainerImageLayer) AfterDropTable(ctx context.Context, query *bun.DropTableQuery) error {
	_, err := query.DB().NewDropIndex().IfExists().Model(l).Index("digest_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "layers" on column "digest" succeeded ✔︎`)
	return nil
}
