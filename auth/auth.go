package auth

import (
	"github.com/jay-dee7/parachute/cache"
	"github.com/jay-dee7/parachute/config"
	"github.com/labstack/echo/v4"
)

type Authentication interface {
	SignUp(ctx echo.Context) error
	SignIn(ctx echo.Context) error
	BasicAuth(username, password string) (map[string]interface{}, error)
}

type auth struct {
	store cache.Store
	c     *config.RegistryConfig
}

func New(s cache.Store, c *config.RegistryConfig) Authentication {
	a := &auth{store: s, c: c}
	return a
}
