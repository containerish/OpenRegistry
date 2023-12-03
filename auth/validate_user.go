package auth

import (
	"context"
	"fmt"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (a *auth) validateUser(username, password string) (*types.User, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	user, err := a.userStore.GetUserByUsername(context.Background(), username)
	if err != nil {
		a.logger.Debug().Err(err).Send()
		return nil, err
	}

	if !a.verifyPassword(user.Password, password) {
		return nil, fmt.Errorf("password is incorrect")
	}

	return user, nil
}
