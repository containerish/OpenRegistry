package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
)

func (a *auth) BasicAuth(username, password string) (map[string]interface{}, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("Email/Password cannot be empty")
	}

	key := fmt.Sprintf("%s/%s", UserNameSpace, username)
	bz, err := a.store.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var userFromDb User
	if err := json.Unmarshal(bz, &userFromDb); err != nil {
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
		"issued_at":  time.Now().Unix(),
	}, nil
}
