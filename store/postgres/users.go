package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
)

func (p *pg) AddUser(ctx context.Context, u *types.User) error {
	if err := u.Validate(); err != nil {
		return err
	}

	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	t := time.Now()
	id := uuid.New()
	_, err := p.conn.Exec(childCtx, queries.AddUser, id.String(), true, u.Username, u.Email, u.Password, t, t)
	if err != nil {
		return fmt.Errorf("error adding user to database: %w", err)
	}

	return nil
}

func (p *pg) GetUser(ctx context.Context, identifier string) (*types.User, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetUser, identifier)

	var user types.User
	err := row.Scan(&user.Username, &user.IsActive, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (p *pg) UpdateUser(identifier string, u *types.User) error {
	return nil
}

func (p *pg) DeleteUser(identifier string) error {
	return nil
}

func (p *pg) IsActive(identifier string) bool {
	return false
}
