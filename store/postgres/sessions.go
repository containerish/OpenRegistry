package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres/queries"
	v2_types "github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/jackc/pgx/v4"
)

func (p *pg) AddSession(ctx context.Context, id, refreshToken, username string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.AddSession, id, refreshToken, username)
	if err != nil {
		return fmt.Errorf("ERR_CREATE_SESSION: %w", err)
	}
	return nil
}

func (p *pg) GetSession(ctx context.Context, sessionId string) (*v2_types.Session, error) {
	childCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	row := p.conn.QueryRow(childCtx, queries.GetSession, sessionId)
	var session v2_types.Session
	if err := row.Scan(&session.Id, &session.RefreshToken, &session.OwnerID); err != nil || err == pgx.ErrNoRows {
		return nil, fmt.Errorf("ERROR_SESSION_LOOKUP: %w", err)
	}
	return &session, nil
}

func (p *pg) DeleteSession(ctx context.Context, sessionId, userId string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.DeleteSession, sessionId, userId)
	if err != nil {
		return fmt.Errorf("ERR_DELETE_SESSION: %w", err)
	}
	return nil
}

func (p *pg) DeleteAllSessions(ctx context.Context, userId string) error {
	childCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := p.conn.Exec(childCtx, queries.DeleteAllSessions, userId)
	if err != nil {
		return fmt.Errorf("ERR_DELETE_ALL_SESSIONS: %w", err)
	}
	return nil
}
