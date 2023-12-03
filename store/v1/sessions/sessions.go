package sessions

import (
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/uptrace/bun"
)

func New(db *bun.DB) users.SessionStore {
	return &sessionStore{
		db,
	}
}
