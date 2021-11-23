package auth

import (
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

// Authentication interface defines the behaviour for container registry and general authentication for the backend
type Authentication interface {
	SignUp(ctx echo.Context) error
	SignIn(ctx echo.Context) error
	BasicAuth() echo.MiddlewareFunc
	Token(ctx echo.Context) error
	JWT() echo.MiddlewareFunc
	ACL() echo.MiddlewareFunc
}

type auth struct {
	pgStore postgres.PersistentStore
	store  cache.Store
	c      *config.RegistryConfig
	logger telemetry.Logger
}

// New is the constructor function returns an Authentication implementation
func New(s cache.Store, c *config.RegistryConfig, pgStore postgres.PersistentStore, logger telemetry.Logger) Authentication {
	a := &auth{store: s, c: c, pgStore: pgStore, logger: logger}
	return a
}
