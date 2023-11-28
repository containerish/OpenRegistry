package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
)

func (a *auth) validateUser(username, password string) (map[string]interface{}, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	userFromDb, err := a.userStore.GetUserByUsername(context.Background(), username)
	if err != nil {
		a.logger.Debug().Err(err).Send()
		return nil, err
	}

	if !a.verifyPassword(userFromDb.Password, password) {
		return nil, fmt.Errorf("password is incorrect")
	}

	token, err := a.newToken(userFromDb)
	if err != nil {
		return nil, err
	}

	return echo.Map{
		"token":      token,
		"expires_in": time.Now().Add(time.Hour).Unix(), // look at auth/jwt.go:251
		"issued_at":  time.Now(),
	}, nil
}
