package types

import (
	"context"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type (
	Permissions struct {
		bun.BaseModel `bun:"table:permissions,alias:p" json:"-"`

		UpdatedAt      time.Time `bun:"updated_at" json:"updated_at,omitempty"`
		CreatedAt      time.Time `bun:"created_at" json:"created_at,omitempty"`
		User           *User     `bun:"rel:belongs-to,join:user_id=id" json:"user"`
		Organization   *User     `bun:"rel:belongs-to,join:organization_id=id" json:"-"`
		UserID         uuid.UUID `bun:"user_id,type:uuid" json:"user_id"`
		OrganizationID uuid.UUID `bun:"organization_id,type:uuid" json:"organization_id"`
		Push           bool      `bun:"push" json:"push"`
		Pull           bool      `bun:"pull" json:"pull"`
		IsAdmin        bool      `bun:"is_admin" json:"is_admin"`
	}

	MigrateToOrgRequest struct {
		UserID uuid.UUID `json:"user_id"`
	}

	RemoveUserFromOrgRequest struct {
		UserID         uuid.UUID `json:"user_id"`
		OrganizationID uuid.UUID `json:"organization_id"`
	}

	AddUsersToOrgRequest struct {
		Users []struct {
			ID      uuid.UUID `json:"id"`
			Pull    bool      `json:"pull"`
			Push    bool      `json:"push"`
			IsAdmin bool      `json:"is_admin"`
		} `json:"users"`
		OrganizationID uuid.UUID `json:"organization_id"`
	}
)

var _ bun.AfterCreateTableHook = (*Permissions)(nil)
var _ bun.AfterDropTableHook = (*Permissions)(nil)
var _ bun.BeforeAppendModelHook = (*Permissions)(nil)

func (p *Permissions) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		p.CreatedAt = time.Now()
	case *bun.UpdateQuery:
		p.UpdatedAt = time.Now()
	}

	return nil
}

func (p *Permissions) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.
		DB().
		NewCreateIndex().
		IfNotExists().
		Model(p).
		Index("org_id_user_id_idx").
		Column("user_id").
		Column("organization_id").
		Exec(ctx)
	if err != nil {
		return err
	}

	color.Yellow(`Create composite index in table "permissions" on columns "user_id" and "organization_id" succeeded ✔︎`)
	return nil
}

func (p *Permissions) AfterDropTable(ctx context.Context, query *bun.DropTableQuery) error {
	_, err := query.DB().NewDropIndex().IfExists().Model(p).Index("org_id_user_id_idx").Exec(ctx)
	if err != nil {
		return err
	}
	color.Yellow(`Drop composite index in table "permissions" on columns "user_id" and "organization_id" succeeded ✔︎`)
	return nil
}
