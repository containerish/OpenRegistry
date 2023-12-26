package users

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type userStore struct {
	db     *bun.DB
	logger telemetry.Logger
}

func New(bunWrappedDB *bun.DB, logger telemetry.Logger) UserStore {
	store := &userStore{
		db:     bunWrappedDB,
		logger: logger,
	}

	return store
}

type UserStore interface {
	v2.PgTxnHandler
	UserWriter
	UserReader
	UserDeleter
}

type UserReader interface {
	UserGetter

	GetIPFSUser(ctx context.Context) (*types.User, error)
	GetUserByIDWithTxn(ctx context.Context, id uuid.UUID, txn *bun.Tx) (*types.User, error)
	GetUserByUsernameWithTxn(ctx context.Context, username string, txn *bun.Tx) (*types.User, error)
	GetGitHubUser(ctx context.Context, identifier string, txn *bun.Tx) (*types.User, error)
	IsActive(ctx context.Context, identifier uuid.UUID) bool
	GetOrgAdmin(ctx context.Context, orgID uuid.UUID) (*types.User, error)
	Search(ctx context.Context, query string) ([]*types.User, error)
	GetOrgUsersByOrgID(ctx context.Context, orgID uuid.UUID) ([]*types.Permissions, error)
	MatchUserType(ctx context.Context, userType types.UserType, userIds ...uuid.UUID) bool
	AddAuthToken(ctx context.Context, token *types.AuthTokens) error
	ListAuthTokens(ctx context.Context, ownerID uuid.UUID) ([]*types.AuthTokens, error)
	GetAuthToken(ctx context.Context, ownerID uuid.UUID, hashedToken string) (*types.AuthTokens, error)
}

type UserGetter interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*types.User, error)
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	GetUserWithSession(ctx context.Context, sessionId string) (*types.User, error)
	UserExists(ctx context.Context, username, email string) (bool, bool)
}

type UserWriter interface {
	AddUser(ctx context.Context, u *types.User, txn *bun.Tx) error
	UpdateUser(ctx context.Context, u *types.User) (*types.User, error)
	UpdateUserPWD(ctx context.Context, identifier uuid.UUID, newPassword string) error
	ConvertUserToOrg(ctx context.Context, userID uuid.UUID) error
}

type UserDeleter interface {
	DeleteUser(ctx context.Context, identifier uuid.UUID) error
}

type SessionStore interface {
	DeleteSession(ctx context.Context, sessionID, userID uuid.UUID) error
	DeleteAllSessions(ctx context.Context, userID uuid.UUID) error
	AddSession(ctx context.Context, sessionID uuid.UUID, refreshToken string, ownerID uuid.UUID) error
}

type EmailStore interface {
	DeleteVerifyEmail(ctx context.Context, userID uuid.UUID) error
	AddVerifyEmail(ctx context.Context, userId uuid.UUID, token uuid.UUID) error
	GetVerifyEmail(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}
