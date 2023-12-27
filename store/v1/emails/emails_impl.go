package emails

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type emailStore struct {
	db *bun.DB
}

func New(db *bun.DB) users.EmailStore {
	return &emailStore{
		db,
	}
}

func (es *emailStore) AddVerifyEmail(ctx context.Context, userID uuid.UUID, token uuid.UUID) error {
	email := &types.Email{
		Token:  token,
		UserId: userID,
	}

	if _, err := es.db.NewInsert().Model(email).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (es *emailStore) DeleteVerifyEmail(ctx context.Context, userID uuid.UUID) error {
	if _, err := es.db.NewDelete().Model(&types.Email{}).Where("user_id = ?", userID).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

func (es *emailStore) GetVerifyEmail(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var email *types.Email

	if err := es.db.NewSelect().Model(email).Where("user_id = ?", userID).Scan(ctx); err != nil {
		return uuid.UUID{}, v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return email.Token, nil
}
