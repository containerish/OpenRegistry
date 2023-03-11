package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

func (p *pg) AddUser(ctx context.Context, u *types.User, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	t := time.Now()
	if u.Id == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("error creating id for add user: %w", err)
		}
		u.Id = id.String()
	}

	if txn != nil {
		_, err := txn.Exec(
			childCtx,
			queries.AddUser,
			u.Id,
			u.IsActive,
			u.Username,
			u.Name,
			u.Email,
			u.Password,
			u.WebauthnConnected,
			u.GithubConnected,
			u.Hireable,
			u.HTMLURL,
			t,
			t,
		)
		if err != nil {
			return fmt.Errorf("error adding user to database with transaction: %w", err)
		}

		return nil
	}

	_, err := p.conn.Exec(
		childCtx,
		queries.AddUser,
		u.Id,
		u.IsActive,
		u.Username,
		u.Name,
		u.Email,
		u.Password,
		u.Hireable,
		u.HTMLURL,
		t,
		t,
	)
	if err != nil {
		return fmt.Errorf("error adding user to database: %w", err)
	}

	return nil
}

func (p *pg) AddOAuthUser(ctx context.Context, u *types.User) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	t := time.Now()
	if u.Id == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("error creating id for oauth user")
		}
		u.Id = id.String()
	}

	_, err := p.conn.Exec(
		childCtx,
		queries.AddOAuthUser,
		u.Id,
		u.Username,
		u.Email,
		u.GithubConnected,
		u.HTMLURL,
		t,
		t,
		u.Bio,
		u.Type,
		u.GravatarID,
		u.Login,
		u.Name,
		u.NodeID,
		u.AvatarURL,
		u.OAuthID,
		u.IsActive,
		u.Hireable,
	)
	if err != nil {
		return fmt.Errorf("error adding user to database: %w", err)
	}

	return nil
}

// GetUser returns a types.User. Any of the following parameters can be used to querying the user:
// - user id
// - user email
// - user's username
// It also takes an optional txn field, which can be helpful to query this information from uncommited txns
func (p *pg) GetUser(ctx context.Context, identifier string, withPassword bool, txn pgx.Tx) (*types.User, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	queryRow := p.conn.QueryRow
	if txn != nil {
		queryRow = txn.QueryRow
	}

	var user types.User
	if withPassword {
		row := queryRow(childCtx, queries.GetUserWithPassword, identifier)

		err := row.Scan(
			&user.Id,
			&user.IsActive,
			&user.Username,
			&user.Email,
			&user.Password,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.WebauthnConnected,
			&user.GithubConnected,
		)
		if err != nil {
			return nil, fmt.Errorf("ERR_GET_USER_WITH_PASSWORD_FROM_DB: %w", err)
		}

		return &user, nil
	}

	row := queryRow(childCtx, queries.GetUser, identifier)
	err := row.Scan(
		&user.Id,
		&user.IsActive,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.WebauthnConnected,
		&user.GithubConnected,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_USER_FROM_DB: %w", err)
	}

	return &user, nil
}

// GetUser returns a types.User. Any of the following parameters can be used to querying the user:
// - user id
// - user email
// - user's username
// It also takes an optional txn field, which can be helpful to query this information from uncommited txns
func (p *pg) GetOAuthUser(ctx context.Context, identifier string, txn pgx.Tx) (*types.User, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	queryRow := p.conn.QueryRow
	if txn != nil {
		queryRow = txn.QueryRow
	}

	var user types.User
	row := queryRow(childCtx, queries.GetOAuthUser, identifier)
	err := row.Scan(
		&user.Id,
		&user.Username,
		&user.Email,
		&user.GithubConnected,
		&user.WebauthnConnected,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_OAUTH_USER_FROM_DB: %w", err)
	}

	return &user, nil
}

func (p *pg) UpdateOAuthUser(ctx context.Context, email, login, nodeId string, txn pgx.Tx) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.UpdateOAuthUser, email, login, nodeId)
	if err != nil {
		return fmt.Errorf("ERR_UPDATE_OAUTH_USER_FROM_DB: %w", err)
	}

	return nil
}

// GetUserById returns a types.User. The parameter used to query the user is userID.
// It also takes an optional txn field, which can be helpful to query this information from uncommited txns
func (p *pg) GetUserById(ctx context.Context, userId string, withPassword bool, txn pgx.Tx) (*types.User, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	queryRow := p.conn.QueryRow
	if txn != nil {
		queryRow = txn.QueryRow
	}

	if withPassword {
		row := queryRow(childCtx, queries.GetUserByIdWithPassword, userId)

		var user types.User
		if err := row.Scan(
			&user.Id,
			&user.IsActive,
			&user.Username,
			&user.Email,
			&user.Password,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ERR_GET_USER_BY_ID_PWD_HASH: %w", err)
		}

		return &user, nil
	}

	row := queryRow(childCtx, queries.GetUserById, userId)
	var user types.User
	err := row.Scan(
		&user.Id,
		&user.IsActive,
		&user.Username,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_USER_BY_ID: %w", err)
	}

	return &user, nil
}

func (p *pg) GetUserWithSession(ctx context.Context, sessionId string) (*types.User, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetUserWithSession, sessionId)

	var user types.User
	if err := row.Scan(
		&user.Id,
		&user.IsActive,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.Hireable,
		&user.HTMLURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("ERR_SESSION_NOT_FOUND: %w", err)
	}

	return &user, nil
}

// UpdateUser
// update users set username = $1, email = $2, updated_at = $3 where username = $4
func (p *pg) UpdateUser(ctx context.Context, userId string, u *types.User) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	t := time.Now()
	_, err := p.conn.Exec(childCtx, queries.UpdateUser, u.IsActive, t, userId)
	if err != nil {
		return fmt.Errorf("error updating user: %s", err)
	}
	return nil
}

func (p *pg) UpdateUserPWD(ctx context.Context, identifier string, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("insufficient feilds for updating user")
	}
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.UpdateUserPwd, newPassword, identifier)
	if err != nil {
		return fmt.Errorf("error updating user: %s", err)
	}
	return nil
}

func (p *pg) AddVerifyEmail(ctx context.Context, token, userId string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()
	_, err := p.conn.Exec(childCtx, queries.AddVerifyUser, token, userId)
	if err != nil {
		return fmt.Errorf("error adding verify link: %w", err)
	}
	return nil
}

func (p *pg) GetVerifyEmail(ctx context.Context, token string) (string, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetVerifyUser, token)
	if row == nil {
		return "", fmt.Errorf("could not find verify token for userId")
	}

	var userId string
	err := row.Scan(&userId)
	if err != nil {
		return "", fmt.Errorf("error scanning verify token: %w", err)
	}

	return userId, nil
}

func (p *pg) DeleteVerifyEmail(ctx context.Context, token string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.DeleteVerifyUser, token)
	if err != nil {
		return fmt.Errorf("error deleting verify token: %w", err)
	}

	return nil
}

// DeleteUser - delete from user where username = $1;
func (p *pg) DeleteUser(ctx context.Context, identifier string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.DeleteUser, identifier)
	if err != nil {
		return fmt.Errorf("error deleting user: %s", identifier)
	}
	return nil
}

// IsActive - if the user has logged in, isActive returns true
// this method is also useful for limiting access of malicious actors
func (p *pg) IsActive(ctx context.Context, identifier string) bool {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()
	row := p.conn.QueryRow(childCtx, queries.GetUser, identifier)
	return row != nil
}

// ID can be either a username or a uuid
// func (p *pg) UserExists(ctx context.Context, id string) bool {
// 	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
// 	defer cancel()
//
// 	var exists bool
// 	if err := p.conn.QueryRow(childCtx, queries.UserExists, id).Scan(&exists); err != nil {
// 		return false
//
// 	}
//
// 	return exists
// }

func (p *pg) githubUserExists(ctx context.Context, username, email string) bool {
	var exists bool
	err := p.conn.QueryRow(ctx, queries.GithubUserExists, username, email).Scan((&exists))
	if err != nil {
		return false
	}

	return exists
}

// returns github or webauthn user exists
func (p *pg) UserExists(ctx context.Context, username, email string) (bool, bool) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()

	return p.githubUserExists(childCtx, email, username), p.WebauthnUserExists(childCtx, email, username)

}
