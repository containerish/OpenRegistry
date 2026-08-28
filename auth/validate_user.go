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

func (a *auth) validateUserWithPAT(ctx context.Context, username, authToken string) (*types.User, error) {
	user, err := a.userStore.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	token, err := (&types.AuthToken{}).FromString(authToken)
	if err != nil {
		return nil, fmt.Errorf("ERR_PARSE_AUTH_TOKEN: %w", err)
	}

	hashedToken, err := GenerateSafeHash([]byte(token.RawString()))
	if err != nil {
		return nil, err
	}

	_, err = a.userStore.GetAuthToken(ctx, user.ID, hashedToken)
	if err != nil {
		return nil, err
	}

	return user, nil

}
