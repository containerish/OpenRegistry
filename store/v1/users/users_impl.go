package users

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	v1 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func (us *userStore) NewTxn(ctx context.Context) (*bun.Tx, error) {
	txn, err := us.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  false,
	})
	return &txn, err
}

func (us *userStore) Abort(ctx context.Context, txn *bun.Tx) error {
	return txn.Rollback()
}

func (us *userStore) Commit(ctx context.Context, txn *bun.Tx) error {
	return txn.Commit()
}

// AddUser implements UserStore.
func (us *userStore) AddUser(ctx context.Context, user *types.User, txn *bun.Tx) error {
	if user.ID.String() == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
		}

		user.ID = id
	}

	execFn := us.db.NewInsert().Model(user)
	if txn != nil {
		execFn = txn.NewInsert().Model(user)
	}

	if _, err := execFn.Exec(ctx); err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}

	return nil
}

// DeleteUser implements UserStore.
func (us *userStore) DeleteUser(ctx context.Context, identifier uuid.UUID) error {
	if _, err := us.db.NewDelete().Model(&types.User{ID: identifier}).WherePK().Exec(ctx); err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	return nil
}

// GetGitHubUser implements UserStore.
func (us *userStore) GetGitHubUser(ctx context.Context, githubEmail string, txn *bun.Tx) (*types.User, error) {
	user := &types.User{}
	q := us.db.NewSelect().Model(user)
	if txn != nil {
		q = txn.NewSelect().Model(user)
	}

	q.Where("coalesce(identities->'github'->>'email', '') = ?", githubEmail)

	if err := q.Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

// GetUser implements UserStore.
func (us *userStore) GetUserByID(ctx context.Context, id uuid.UUID) (*types.User, error) {
	user := &types.User{ID: id}
	if err := us.db.NewSelect().Model(user).WherePK().Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	user := &types.User{}
	if err := us.db.NewSelect().Model(user).Where("username = ?", username).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	user := &types.User{}
	if err := us.db.NewSelect().Model(user).Where("email = ?", email).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) GetUserByIDWithTxn(ctx context.Context, id uuid.UUID, txn *bun.Tx) (*types.User, error) {
	user := &types.User{ID: id}
	if err := txn.NewSelect().NewSelect().Model(user).WherePK().Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) GetUserByUsernameWithTxn(ctx context.Context, username string, txn *bun.Tx) (*types.User, error) {
	user := &types.User{}
	if err := txn.NewSelect().NewSelect().Model(user).Where("username = ?", username).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) GetIPFSUser(ctx context.Context) (*types.User, error) {
	user := &types.User{}
	if err := us.db.NewSelect().Model(user).Where("username = ?", types.SystemUsernameIPFS).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

// GetUserWithSession implements UserStore.
func (us *userStore) GetUserWithSession(ctx context.Context, sessionId string) (*types.User, error) {
	parsedSessionId, err := uuid.Parse(sessionId)
	if err != nil {
		return nil, err
	}

	session := &types.Session{Id: parsedSessionId}
	err = us.
		db.
		NewSelect().
		Model(session).
		Relation("User").
		WherePK().
		Scan(ctx)
	if err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return session.User, nil
}

// IsActive implements UserStore.
func (us *userStore) IsActive(ctx context.Context, id uuid.UUID) bool {
	isActive := false
	_ = us.db.NewSelect().Model(&types.User{ID: id}).WherePK().Scan(ctx, &isActive)
	return isActive
}

// UpdateUser implements UserStore.
func (us *userStore) UpdateUser(ctx context.Context, user *types.User) (*types.User, error) {
	if _, err := us.db.NewUpdate().Model(user).WherePK().Exec(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return user, nil
}

// UpdateUserPWD implements UserStore.
func (us *userStore) UpdateUserPWD(ctx context.Context, id uuid.UUID, newPassword string) error {
	_, err := us.db.NewUpdate().Model(&types.User{ID: id}).Set("password = ?", newPassword).WherePK().Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return nil
}

// UserExists implements UserStore.
func (us *userStore) UserExists(ctx context.Context, username string, email string) (bool, bool) {
	return us.githubUserExists(ctx, username, email), us.webAuthnUserExists(ctx, username, email)
}

func (us *userStore) githubUserExists(ctx context.Context, username, email string) bool {
	var exists bool
	err := us.
		db.
		NewSelect().
		Model(&types.User{}).
		Where(
			"identities->'github'->>'email' = ? or identities->'github'->>'username' = ?",
			bun.Ident(email),
			bun.Ident(username),
		).
		Scan(ctx, &exists)
	if err != nil {
		return false
	}

	return exists
}

func (us *userStore) webAuthnUserExists(ctx context.Context, username, email string) bool {
	var exists bool
	err := us.
		db.
		NewSelect().
		Model(&types.User{}).
		Where(
			"identities->'webauthn'->>'email' = ? or identities->'webauthn'->>'username' = ?",
			bun.Ident(email),
			bun.Ident(username),
		).
		Scan(ctx, &exists)
	if err != nil {
		return false
	}

	return exists
}

func (us *userStore) ConvertUserToOrg(ctx context.Context, userID uuid.UUID) error {
	user := &types.User{ID: userID}

	_, err := us.
		db.
		NewUpdate().
		Model(user).
		WherePK().
		Set("user_type = ?", types.UserTypeOrganization.String()).
		Set("is_org_owner = ?", true).
		Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return nil
}

func (us *userStore) GetOrgAdmin(ctx context.Context, orgID uuid.UUID) (*types.User, error) {
	user := &types.User{ID: orgID}

	if err := us.
		db.
		NewSelect().
		Model(user).
		WherePK().
		Where("user_type = ?", types.UserTypeOrganization.String()).
		Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return user, nil
}

func (us *userStore) Search(ctx context.Context, query string) ([]*types.User, error) {
	users := []*types.User{}

	b := strings.Builder{}
	b.WriteString("%")
	b.WriteString(query)
	b.WriteString("%")

	q := us.
		db.
		NewSelect().
		Model(&users).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("user_type = ?", types.UserTypeRegular.String()) // only search for users (skip orgs and system users
		}).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.WhereOr("username ilike ?", b.String()).
				WhereOr("email ilike ?", b.String())
		}).
		ExcludeColumn("password").
		ExcludeColumn("updated_at").
		ExcludeColumn("created_at").
		ExcludeColumn("is_active").
		Limit(types.DefaultSearchLimit)

	err := q.Scan(ctx)
	if err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return users, nil
}

// GetOrgUsersByOrgID returns a list of Permission structs, which also has the user to which the permissions belongs to
func (us *userStore) GetOrgUsersByOrgID(ctx context.Context, orgID uuid.UUID) ([]*types.Permissions, error) {
	var perms []*types.Permissions

	q := us.db.NewSelect().Model(&perms).Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
		return sq.ExcludeColumn("password")
	}).Where("organization_id = ?", orgID)

	if err := q.Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return perms, nil
}

func (us *userStore) MatchUserType(ctx context.Context, userType types.UserType, userIds ...uuid.UUID) bool {
	var users []*types.User

	q := us.
		db.
		NewSelect().
		Model(&users).
		Where("user_type = ?", types.UserTypeRegular.String()).
		Where("id in (?)", bun.In(userIds))

	count, err := q.Count(ctx)
	if err != nil {
		return false
	}

	return len(userIds) == count
}

func (us *userStore) AddAuthToken(ctx context.Context, token *types.AuthTokens) error {
	if token.ExpiresAt.IsZero() {
		token.ExpiresAt = time.Now().AddDate(1, 0, 0)
	}

	_, err := us.db.NewInsert().Model(token).Exec(ctx)
	return err
}

func (us *userStore) ListAuthTokens(ctx context.Context, ownerID uuid.UUID) ([]*types.AuthTokens, error) {
	var tokens []*types.AuthTokens

	err := us.
		db.
		NewSelect().
		Model(&tokens).
		ExcludeColumn("auth_token").
		ExcludeColumn("owner_id").
		Where("owner_id = ?", ownerID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (us *userStore) GetAuthToken(
	ctx context.Context,
	ownerID uuid.UUID,
	hashedToken string,
) (*types.AuthTokens, error) {
	var token types.AuthTokens

	err := us.
		db.
		NewSelect().
		Model(&token).
		Where("owner_id = ?", ownerID).
		Where("auth_token = ?", hashedToken).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	if token.ExpiresAt.Unix() < time.Now().Unix() {
		return nil, fmt.Errorf("token has expired, please generate a new one")
	}

	return &token, nil
}
