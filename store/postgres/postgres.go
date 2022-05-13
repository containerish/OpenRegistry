package postgres

import (
	"context"
	"github.com/duo-labs/webauthn/webauthn"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type PersistentStore interface {
	UserStore
	RegistryStore
	SessionStore
	WebAuthN
	Close()
}

type UserStore interface {
	AddUser(ctx context.Context, u *types.User) error
	AddOAuthUser(ctx context.Context, u *types.User) error
	UserExists(ctx context.Context, id string) bool
	GetUser(ctx context.Context, identifier string, wihtPassword bool) (*types.User, error)
	GetUserById(ctx context.Context, userId string, wihtPassword bool) (*types.User, error)
	GetUserWithSession(ctx context.Context, sessionId string) (*types.User, error)
	UpdateUser(ctx context.Context, identifier string, u *types.User) error
	UpdateUserPWD(ctx context.Context, identifier string, newPassword string) error
	DeleteUser(ctx context.Context, identifier string) error
	IsActive(ctx context.Context, identifier string) bool
	AddSession(ctx context.Context, sessionId, refreshToken, owner string) error
	DeleteSession(ctx context.Context, sessionId, userId string) error
	DeleteAllSessions(ctx context.Context, userId string) error
	AddVerifyEmail(ctx context.Context, userId, token string) error
	GetVerifyEmail(ctx context.Context, userId string) (string, error)
	DeleteVerifyEmail(ctx context.Context, userId string) error
}

type RegistryStore interface {
	NewTxn(ctx context.Context) (pgx.Tx, error)
	Abort(ctx context.Context, txn pgx.Tx) error
	Commit(ctx context.Context, txn pgx.Tx) error
	SetLayer(ctx context.Context, txn pgx.Tx, l *types.LayerV2) error
	SetManifest(ctx context.Context, txn pgx.Tx, im *types.ImageManifestV2) error
	SetBlob(ctx context.Context, txn pgx.Tx, b *types.Blob) error
	SetConfig(ctx context.Context, txn pgx.Tx, cfg types.ConfigV2) error
	GetManifest(ctx context.Context, ref string) (*types.ImageManifestV2, error)
	GetManifestByReference(ctx context.Context, namespace string, ref string) (*types.ConfigV2, error)
	GetLayer(ctx context.Context, digest string) (*types.LayerV2, error)
	GetContentHashById(ctx context.Context, uuid string) (string, error)
	GetBlob(ctx context.Context, digest string) ([]*types.Blob, error)
	GetConfig(ctx context.Context, namespace string) ([]*types.ConfigV2, error)
	GetImageTags(ctx context.Context, namespace string) ([]string, error)
	GetCatalog(ctx context.Context, namespace string, pageSize int64, offset int64) ([]string, error)
	GetCatalogDetail(
		ctx context.Context, namespace string, pageSize int64, offset int64, sortBy string,
	) ([]*types.ImageManifestV2, error)
	GetRepoDetail(ctx context.Context, namespace string, pageSize int64, offset int64) (*types.Repository, error)
	GetCatalogCount(ctx context.Context, ns string) (int64, error)
	GetImageNamespace(ctx context.Context, search string) ([]*types.ImageManifestV2, error)
	DeleteLayerV2(ctx context.Context, txn pgx.Tx, digest string) error
	DeleteBlobV2(ctx context.Context, txn pgx.Tx, digest string) error
	DeleteManifestOrTag(ctx context.Context, txn pgx.Tx, reference string) error
}

type SessionStore interface {
	AddSession(ctx context.Context, id, refreshToken, username string) error
	GetSession(ctx context.Context, sessionId string) (*types.Session, error)
	DeleteSession(ctx context.Context, sessionId, userId string) error
	DeleteAllSessions(ctx context.Context, userId string) error
}

type WebAuthN interface {
	GetWebAuthNCredentials(ctx context.Context, id string) (*webauthn.Credential, error)
	AddWebAuthNCredentials(ctx context.Context, credential *webauthn.Credential) error
	GetWebAuthNSessionData(ctx context.Context, userId string) (*webauthn.SessionData, error)
	AddWebAuthSessionData(ctx context.Context, sessionData *webauthn.SessionData) error
}

type pg struct {
	conn *pgxpool.Pool
}

func (p *pg) Close() {
	p.conn.Close()
}

func New(cfg *config.Store) (PersistentStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	pgxCofig, err := pgxpool.ParseConfig(cfg.Endpoint())
	if err != nil {
		return nil, err
	}

	conn, err := pgxpool.ConnectConfig(ctx, pgxCofig)
	if err != nil {
		return nil, err
	}

	color.Green("connection to database successful")
	return &pg{conn: conn}, nil
}

const (
	ErrDuplicateConstraintUsername = "username_key"
	ErrDuplicateConstraintEmail    = "email_key"
)
