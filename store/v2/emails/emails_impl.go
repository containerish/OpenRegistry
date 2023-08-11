package emails

import (
	"context"

	v2 "github.com/containerish/OpenRegistry/store/v2"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/store/v2/users"
	"github.com/uptrace/bun"
)

type emailStore struct {
	db *bun.DB
}

func NewStore(db *bun.DB) users.EmailStore {
	return &emailStore{
		db,
	}
}

func (es *emailStore) AddVerifyEmail(ctx context.Context, userId string, token string) error {
	email := &types.Email{
		Token:  token,
		UserId: userId,
	}

	if _, err := es.db.NewInsert().Model(email).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationWrite)
	}

	return nil
}

func (es *emailStore) DeleteVerifyEmail(ctx context.Context, userId string) error {
	if _, err := es.db.NewDelete().Model(&types.Email{}).Where("user_id = ?1", userId).Exec(ctx); err != nil {
		return v2.WrapDatabaseError(err, v2.DatabaseOperationDelete)
	}

	return nil
}

func (es *emailStore) GetVerifyEmail(ctx context.Context, userId string) (string, error) {
	var token string
	if err := es.db.NewSelect().Model(&types.Email{}).Where("user_id = ?1", userId).Scan(ctx, &token); err != nil {
		return "", v2.WrapDatabaseError(err, v2.DatabaseOperationRead)
	}

	return token, nil
}
