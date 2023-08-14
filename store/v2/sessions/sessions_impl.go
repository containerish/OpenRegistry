package sessions

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type sessionStore struct {
	db *bun.DB
}

// DeleteAllSessions implements UserStore.
func (ss *sessionStore) DeleteAllSessions(ctx context.Context, userID uuid.UUID) error {
	if _, err := ss.db.NewDelete().Model(&types.Session{}).Where("owner_id = ?", userID).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

// DeleteSession implements UserStore.
func (ss *sessionStore) DeleteSession(ctx context.Context, sessionId uuid.UUID, userID uuid.UUID) error {
	if _, err := ss.db.NewDelete().Model(&types.Session{Id: sessionId}).WherePK().Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

// AddSession implements UserStore.
func (ss *sessionStore) AddSession(ctx context.Context, sessionID uuid.UUID, refreshToken string, ownerID uuid.UUID) error {
	session := &types.Session{
		Id:           sessionID,
		RefreshToken: refreshToken,
		OwnerID:      ownerID,
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
