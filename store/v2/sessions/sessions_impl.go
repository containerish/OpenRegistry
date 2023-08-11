package sessions

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/uptrace/bun"
)

type sessionStore struct {
	db *bun.DB
}

// DeleteAllSessions implements UserStore.
func (ss *sessionStore) DeleteAllSessions(ctx context.Context, userId string) error {
	if _, err := ss.db.NewDelete().Model(&types.Session{}).Where("owner_id = ?", userId).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

// DeleteSession implements UserStore.
func (ss *sessionStore) DeleteSession(ctx context.Context, sessionId string, userId string) error {
	if _, err := ss.db.NewDelete().Model(&types.Session{}).WherePK(sessionId).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

// AddSession implements UserStore.
func (ss *sessionStore) AddSession(ctx context.Context, sessionId string, refreshToken string, owner string) error {
	session := &types.Session{
		Id:           sessionId,
		RefreshToken: refreshToken,
		OwnerID:      owner,
	}

	if _, err := ss.db.NewInsert().Model(session).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (ss *sessionStore) GetSession(ctx context.Context, sessionId string) (*types.Session, error) {
	session := &types.Session{}
	if _, err := ss.db.NewSelect().Model(session).WherePK(sessionId).Exec(ctx); err != nil {
		return nil, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return session, nil
}
