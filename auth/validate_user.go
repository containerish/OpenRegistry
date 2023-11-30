package auth

import (
	"context"
	"fmt"
)

func (a *auth) validateUser(username, password string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	userFromDb, err := a.userStore.GetUserByUsername(context.Background(), username)
	if err != nil {
		a.logger.Debug().Err(err).Send()
		return err
	}

	if !a.verifyPassword(userFromDb.Password, password) {
		return fmt.Errorf("password is incorrect")
	}

	return nil

	// token, err := a.newToken(userFromDb)
	// if err != nil {
	// 	return nil, err
	// }

	// return echo.Map{
	// 	"token":      token,
	// 	"expires_in": time.Now().Add(time.Hour).Unix(), // look at auth/jwt.go:251
	// 	"issued_at":  time.Now(),
	// }, nil
}
