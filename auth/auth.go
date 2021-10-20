package auth

import (
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	"github.com/labstack/echo/v4"
)

// Authentication interface defines the behaviour for container registry and general authentication for the backend
type Authentication interface {
	SignUp(ctx echo.Context) error
	SignIn(ctx echo.Context) error
	BasicAuth() echo.MiddlewareFunc
	Token(ctx echo.Context) error
}

type auth struct {
	store cache.Store
	c     *config.RegistryConfig
}

// New is the constructor function returns an Authentication implementation
func New(s cache.Store, c *config.RegistryConfig) Authentication {
	a := &auth{store: s, c: c}
	return a
}
