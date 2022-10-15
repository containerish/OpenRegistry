package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
)

func (a *auth) validateUser(username, password string) (map[string]interface{}, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("Email/Password cannot be empty")
	}

	userFromDb, err := a.pgStore.GetUser(context.Background(), username, true, nil)
	if err != nil {
		return nil, err
	}

	if !a.verifyPassword(userFromDb.Password, password) {
		return nil, fmt.Errorf("invalid password")
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
