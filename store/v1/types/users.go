package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type (
	User struct {
		bun.BaseModel `bun:"table:users,alias:u" json:"-"`

		UpdatedAt           time.Time                   `bun:"updated_at" json:"updated_at,omitempty" validate:"-"`
		CreatedAt           time.Time                   `bun:"created_at" json:"created_at,omitempty" validate:"-"`
		Identities          Identities                  `bun:"identities" json:"identities,omitempty"`
		Username            string                      `bun:"username,notnull,unique" json:"username,omitempty" validate:"-"`
		Password            string                      `bun:"password" json:"password,omitempty"`
		Email               string                      `bun:"email,notnull,unique" json:"email,omitempty" validate:"email"`
		Repositories        []*ContainerImageRepository `bun:"rel:has-many,join:id=owner_id" json:"-"`
		Sessions            []*Session                  `bun:"rel:has-many,join:id=owner_id" json:"-"`
		WebauthnSessions    []*WebauthnSession          `bun:"rel:has-many,join:id=user_id" json:"-"`
		WebauthnCredentials []*WebauthnCredential       `bun:"rel:has-many,join:id=credential_owner_id" json:"-"`
		ID                  uuid.UUID                   `bun:"id,type:uuid,pk" json:"id,omitempty" validate:"-"`
		IsActive            bool                        `bun:"is_active" json:"is_active,omitempty" validate:"-"`
		WebauthnConnected   bool                        `bun:"webauthn_connected" json:"webauthn_connected"`
		GithubConnected     bool                        `bun:"github_connected" json:"github_connected"`
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
)

const (
	UserContextKey           ContextKey = "UserContextKey"
	UserClaimsContextKey     ContextKey = "UserClaimsContextKey"
	UserRepositoryContextKey ContextKey = "UserRepositoryContextKey"
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