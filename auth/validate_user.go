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

	userFromDb, err := a.pgStore.GetUser(context.Background(), username)
	if err != nil {
		return nil, err
	}

	if !a.verifyPassword(userFromDb.Password, password) {
		return nil, fmt.Errorf("invalid password")
	}

	tokenLife := time.Now().Add(time.Hour * 24 * 14).Unix()
	token, err := a.newToken(User{Username: username}, tokenLife)
	if err != nil {
		return nil, err
	}

	return echo.Map{
		"token":      token,
		"expires_in": tokenLife,
		"issued_at":  time.Now(),
	}, nil
}
