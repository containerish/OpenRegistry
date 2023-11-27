package users

import (
	"context"
	"database/sql"

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
	selectFn := us.db.NewSelect().Model(user)
	if txn != nil {
		selectFn = txn.NewSelect().Model(user)
	}

	if err := selectFn.Where("coalesce(identities->'github'->>'email', '') = ?", githubEmail).Scan(ctx); err != nil {
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
	var user types.User
	if err := us.db.NewSelect().Model(&user).Where("username = ?", types.RepositoryNameIPFS).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return &user, nil
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
			"identities->'github'->>'email' = ?1 or identities->'github'->>'username' = ?",
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
			"identities->'webauthn'->>'email' = ?1 or identities->'webauthn'->>'username' = ?",
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
	user := types.User{ID: userID}

	_, err := us.
		db.
		NewUpdate().
		Model(&user).
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
