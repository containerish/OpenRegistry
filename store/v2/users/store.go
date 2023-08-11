package users

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/uptrace/bun"
)

type userStore struct {
	db     *bun.DB
	logger telemetry.Logger
}

func NewStore(bunWrappedDB *bun.DB, logger telemetry.Logger) UserStore {
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
	GetUserByID(ctx context.Context, userId string) (*types.User, error)
	GetUserByIDWithTxn(ctx context.Context, id string, txn *bun.Tx) (*types.User, error)
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)
	GetUserByUsernameWithTxn(ctx context.Context, username string, txn *bun.Tx) (*types.User, error)
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	GetUserWithSession(ctx context.Context, sessionId string) (*types.User, error)
	GetGitHubUser(ctx context.Context, identifier string, txn *bun.Tx) (*types.User, error)
	IsActive(ctx context.Context, identifier string) bool
	// ID can be either a username, oauth login (GitHub username) or the user id (uuid)
	UserExists(ctx context.Context, username, email string) (bool, bool)
}

type UserWriter interface {
	AddUser(ctx context.Context, u *types.User, txn *bun.Tx) error
	UpdateUser(ctx context.Context, u *types.User) (*types.User, error)
	UpdateUserPWD(ctx context.Context, identifier string, newPassword string) error
}

type UserDeleter interface {
	DeleteUser(ctx context.Context, identifier string) error
}

type SessionStore interface {
	DeleteSession(ctx context.Context, sessionId, userId string) error
	DeleteAllSessions(ctx context.Context, userId string) error
	AddSession(ctx context.Context, sessionId, refreshToken, owner string) error
}

type EmailStore interface {
	DeleteVerifyEmail(ctx context.Context, userId string) error
	AddVerifyEmail(ctx context.Context, userId, token string) error
	GetVerifyEmail(ctx context.Context, userId string) (string, error)
}
