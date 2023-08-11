package sessions

import (
	"github.com/containerish/OpenRegistry/store/v2/users"
	"github.com/uptrace/bun"
)

func NewStore(db *bun.DB) users.SessionStore {
	return &sessionStore{
		db,
	}
}
