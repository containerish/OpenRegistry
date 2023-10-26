package types

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
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
		ImageManifestUUID string               `json:"image_manifest_uuid"`
		Visibility        RepositoryVisibility `json:"visibility_mode"`
	}

	ImageManifest struct {
		bun.BaseModel `bun:"table:image_manifests,alias:m" json:"-"`
		CreatedAt     time.Time                 `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
		UpdatedAt     time.Time                 `bun:"updated_at,nullzero" json:"updated_at"`
		Repository    *ContainerImageRepository `bun:"rel:belongs-to,join:repository_id=id" json:"-"`
		User          *User                     `bun:"rel:belongs-to,join:owner_id=id" json:"-"`
		Subject       map[string]any            `bun:"subject,type:jsonb" json:"subject"`
		Digest        string                    `bun:"digest,notnull" json:"digest"`
		Reference     string                    `bun:"reference,notnull" json:"reference"`
		MediaType     string                    `bun:"media_type,notnull" json:"media_type"`
		DFSLink       string                    `bun:"dfs_link,notnull" json:"dfs_link"`
		Layers        []string                  `bun:"layers,array" json:"layers"`
		SchemaVersion int                       `bun:"schema_version,notnull" json:"schema_version"`
		Size          uint64                    `bun:"size,notnull" json:"size"`
		RepositoryID  uuid.UUID                 `bun:"repository_id,type:uuid" json:"repository_id"`
		ID            uuid.UUID                 `bun:"id,pk,type:uuid" json:"id"`
		OwnerID       uuid.UUID                 `bun:"owner_id,type:uuid" json:"owner_id"`
	}

	ContainerImageLayer struct {
		bun.BaseModel `bun:"table:layers,alias:l" json:"-"`

		CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
		UpdatedAt time.Time `bun:"updated_at,nullzero" json:"updated_at"`
		ID        string    `bun:"id,pk,type:uuid" json:"id"`
		Digest    string    `bun:"digest,notnull,unique" json:"digest"`
		MediaType string    `bun:"media_type,notnull" json:"media_type"`
		DFSLink   string    `bun:"dfs_link" json:"dfs_link"`
		Size      uint64    `bun:"size,default:0" json:"size"`
	}

	ContainerImageRepository struct {
		bun.BaseModel `bun:"table:repositories,alias:r" json:"-"`

		CreatedAt      time.Time            `bun:"created_at" json:"created_at"`
		UpdatedAt      time.Time            `bun:"updated_at" json:"updated_at"`
		MetaTags       map[string]any       `bun:"meta_tags" json:"meta_tags"`
		User           *User                `bun:"rel:belongs-to,join:owner_id=id" json:"-"`
		Project        *RepositoryBuild     `bun:"rel:has-one,join:id=repository_id" json:"-"`
		Description    string               `bun:"description" json:"description"`
		Visibility     RepositoryVisibility `bun:"visibility,notnull" json:"visibility"`
		Name           string               `bun:"name,notnull,unique" json:"name"`
		ImageManifests []*ImageManifest     `bun:"rel:has-many,join:id=repository_id" json:"image_manifests,omitempty"`
		Builds         []*RepositoryBuild   `bun:"rel:has-many,join:id=repository_id" json:"-"`
		ID             uuid.UUID            `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
		OwnerID        uuid.UUID            `bun:"owner_id,type:uuid" json:"owner_id"`
	}

	RepositoryVisibility string

	ReferrerManifest struct {
		Annotations         map[string]string `json:"annotations,omitempty"`
		MediaType           string            `json:"mediaType"`
		Digest              string            `json:"digest"`
		ArtifactType        string            `json:"artifactType,omitempty"`
		NewUnspecifiedField string            `json:"newUnspecifiedField,omitempty"`
		Size                int               `json:"size"`
	}

	Referrer struct {
		MediaType     string             `json:"mediaType"`
		Manifests     []ReferrerManifest `json:"manifests"`
		SchemaVersion int                `json:"schemaVersion"`
	}
)

func (rm *ReferrerManifest) ToGoMap() map[string]any {
	if rm == nil {
		return nil
	}

	m := map[string]any{
		"annotations":         rm.Annotations,
		"mediaType":           rm.MediaType,
		"digest":              rm.Digest,
		"artifactType":        rm.ArtifactType,
		"newUnspecifiedField": rm.NewUnspecifiedField,
		"size":                rm.Size,
	}

	return m
}

func (imf *ImageManifest) GetSubject() *ReferrerManifest {
	if imf.Subject == nil {
		return nil
	}

	bz, err := json.Marshal(imf.Subject)
	if err != nil {
		return nil
	}

	var refManifest ReferrerManifest
	if err = json.Unmarshal(bz, &refManifest); err == nil {
		return &refManifest
	}

	return nil
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
var _ bun.AfterCreateTableHook = (*User)(nil)

func (u *User) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.DB().NewCreateIndex().IfNotExists().Model(u).Index("email_idx").Column("email").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Create index in table "users" on column "email" succeeded ✔︎`)

	_, err = query.DB().NewCreateIndex().IfNotExists().Model(u).Index("username_idx").Column("username").Exec(ctx)
	if err != nil {
		return err
	}

	color.Yellow(`Create index in table "users" on column "username" succeeded ✔︎`)
	return nil
}

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
var _ bun.AfterDropTableHook = (*User)(nil)

func (u *User) AfterDropTable(ctx context.Context, query *bun.DropTableQuery) error {
	_, err := query.DB().NewDropIndex().IfExists().Model(u).Index("email_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "users" on column "email" succeeded ✔︎`)

	_, err = query.DB().NewDropIndex().IfExists().Model(u).Index("username_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop index in table "users" on column "username" succeeded ✔︎`)
	return nil
}

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
