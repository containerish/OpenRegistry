package sessions

import (
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/uptrace/bun"
)

func NewStore(db *bun.DB) users.SessionStore {
	return &sessionStore{
		db,
	}
}
