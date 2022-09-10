package vcs

import (
	"context"

	"github.com/containerish/OpenRegistry/types"
	pgx "github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
)

type VCS interface {
	// ListAuthorisedRepositories returns a JSON array of repositories, which the user has shared access for
	ListAuthorisedRepositories(ctx echo.Context) error
	HandleWebhookEvents(ctx echo.Context) error
	HandleSetupCallback(ctx echo.Context) error
	ListHandlers() []echo.HandlerFunc
	RegisterRoutes(subRouter *echo.Group)
	HandleGithubAppFinish(ctx echo.Context) error
}

type VCSStore interface {
	UpdateInstallationID(ctx context.Context, id, githubUsername string) error
	GetInstallationID(ctx context.Context, githubUsername string) (string, error)
	GetUserById(ctx context.Context, userId string, wihtPassword bool, txn pgx.Tx) (*types.User, error)
}

type Repository struct {
	Owner string
	Name  string
}
