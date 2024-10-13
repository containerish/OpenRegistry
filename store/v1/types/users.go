package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/go-playground/validator/v10"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type UserType int

const (
	UserTypeRegular UserType = iota + 1
	UserTypeOrganization
	UserTypeSystem
)

const (
	DefaultSearchLimit = 25
)

func (ut UserType) String() string {
	switch ut {
	case UserTypeRegular:
		return "user"
	case UserTypeSystem:
		return "system"
	case UserTypeOrganization:
		return "organization"
	default:
		return "user"
	}
}

type (
	User struct {
		bun.BaseModel `bun:"table:users,alias:u" json:"-"`

		UpdatedAt  time.Time  `bun:"updated_at" json:"updated_at,omitempty" validate:"-"`
		CreatedAt  time.Time  `bun:"created_at" json:"created_at,omitempty" validate:"-"`
		Identities Identities `bun:"identities" json:"identities,omitempty"`
		// nolint:lll
		Username string `bun:"username,notnull,unique" json:"username,omitempty" validate:"-"`
		Password string `bun:"password" json:"password,omitempty"`
		// nolint:lll
		Email               string                      `bun:"email,notnull,unique" json:"email,omitempty" validate:"email"`
		UserType            string                      `bun:"user_type" json:"user_type"`
		Sessions            []*Session                  `bun:"rel:has-many,join:id=owner_id" json:"-"`
		WebauthnSessions    []*WebauthnSession          `bun:"rel:has-many,join:id=user_id" json:"-"`
		WebauthnCredentials []*WebauthnCredential       `bun:"rel:has-many,join:id=credential_owner_id" json:"-"`
		Permissions         []*Permissions              `bun:"rel:has-many,join:id=user_id" json:"permissions"`
		Repositories        []*ContainerImageRepository `bun:"rel:has-many,join:id=owner_id" json:"-"`
		Projects            []*RepositoryBuildProject   `bun:"rel:has-many,join:id=repository_owner_id" json:"-"`
		AuthTokens          []*AuthTokens               `bun:"rel:has-many,join:id=owner_id" json:"-"`
		// nolint:lll
		FavoriteRepositories []uuid.UUID `bun:"favorite_repositories,nullzero,type:uuid[],default:'{}'" json:"favorite_repositories"`
		ID                   uuid.UUID   `bun:"id,type:uuid,pk" json:"id,omitempty" validate:"-"`
		IsActive             bool        `bun:"is_active" json:"is_active,omitempty" validate:"-"`
		WebauthnConnected    bool        `bun:"webauthn_connected" json:"webauthn_connected"`
		GithubConnected      bool        `bun:"github_connected" json:"github_connected"`
		IsOrgOwner           bool        `bun:"is_org_owner" json:"is_org_owner,omitempty"`
	}

	AuthTokens struct {
		bun.BaseModel `bun:"table:auth_tokens,alias:s" json:"-"`

		CreatedAt time.Time `bun:"created_at" json:"created_at,omitempty" validate:"-"`
		ExpiresAt time.Time `bun:"expires_at" json:"expires_at,omitempty" validate:"-"`
		Name      string    `bun:"name" json:"name"`
		AuthToken string    `bun:"auth_token,type:text,pk" json:"-"`
		OwnerID   uuid.UUID `bun:"owner_id,type:uuid" json:"-"`
	}

	// type here is string so that we can use it with echo.Context & std context.Context
	ContextKey string

	Session struct {
		bun.BaseModel `bun:"table:sessions,alias:s" json:"-"`

		User         *User     `bun:"rel:belongs-to,join:owner_id=id"`
		RefreshToken string    `bun:"refresh_token" json:"refresh_token"`
		Id           uuid.UUID `bun:"id,type:uuid,pk" json:"id"`
		OwnerID      uuid.UUID `bun:"owner_id,type:uuid" json:"-"`
	}

	Identities   map[string]*UserIdentity
	UserIdentity struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Username       string `json:"username"`
		Email          string `json:"email"`
		Avatar         string `json:"avatar"`
		InstallationID int64  `json:"installation_id"`
	}

	Email struct {
		bun.BaseModel `bun:"table:emails" json:"-"`

		Token  uuid.UUID `bun:"token,pk,type:uuid" json:"-"`
		UserId uuid.UUID `bun:"user_id,type:uuid" json:"-"`
	}

	CreateAuthTokenRequest struct {
		ExpiresAt time.Time `json:"expires_at"`
		Name      string    `json:"name"`
	}
)

const (
	UserContextKey               ContextKey = "UserContextKey"
	UserClaimsContextKey         ContextKey = "UserClaimsContextKey"
	UserRepositoryContextKey     ContextKey = "UserRepositoryContextKey"
	OrgModeRequestBodyContextKey ContextKey = "OrgModeRequestBodyContextKey"
	UserPermissionsContextKey    ContextKey = "UserPermissionsContextKey"
)

func (u *User) Bytes() ([]byte, error) {
	if u == nil {
		return nil, fmt.Errorf("user struct is nil")
	}

	return json.Marshal(u)
}

func (*User) NewUserFromGitHubUser(ghUser github.User) *User {
	return &User{
		ID:              uuid.New(),
		Username:        ghUser.GetLogin(),
		Email:           ghUser.GetEmail(),
		IsActive:        true,
		GithubConnected: true,
		UserType:        UserTypeRegular.String(),
		Identities: map[string]*UserIdentity{
			IdentityProviderGitHub: {
				ID:             fmt.Sprintf("%d", ghUser.GetID()),
				Name:           ghUser.GetName(),
				Username:       ghUser.GetLogin(),
				Email:          ghUser.GetEmail(),
				Avatar:         ghUser.GetAvatarURL(),
				InstallationID: 0,
			},
		},
	}
}

func (i Identities) GetGitHubIdentity() *UserIdentity {
	identity, ok := i[IdentityProviderGitHub]
	if ok {
		return identity
	}

	return nil
}

func (i Identities) GetWebauthnIdentity() *UserIdentity {
	identity, ok := i[IdentityProviderWebauthn]
	if ok {
		return identity
	}

	return nil
}

func (u *User) Validate(validatePassword bool) error {
	if u == nil {
		return fmt.Errorf("user is nil")
	}

	// there's no password for OAuth Users
	if validatePassword {
		if err := ValidatePassword(u.Password); err != nil {
			return fmt.Errorf("invalid password: %w", err)
		}
	}

	v := validator.New()
	return v.Struct(u)
}

const IdentityProviderGitHub = "github"
const IdentityProviderWebauthn = "webauthn"

var _ bun.AfterCreateTableHook = (*User)(nil)
var _ bun.AfterDropTableHook = (*User)(nil)
var _ bun.BeforeAppendModelHook = (*ImageManifest)(nil)

func (u *User) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		u.CreatedAt = time.Now()
	case *bun.UpdateQuery:
		u.UpdatedAt = time.Now()
	}

	return nil
}

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

	// setup any system users
	ipfsUser := &User{
		CreatedAt:  time.Now(),
		Username:   SystemUsernameIPFS,
		UserType:   UserTypeSystem.String(),
		ID:         uuid.New(),
		IsActive:   true,
		IsOrgOwner: true,
	}

	_, err = query.DB().NewInsert().Model(ipfsUser).Exec(ctx)
	if err != nil && !strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
		return err
	}

	color.Yellow(`Create index in table "users" on column "username" succeeded ✔︎`)
	return nil
}

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
