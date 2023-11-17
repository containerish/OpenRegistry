package vcs

import (
	"context"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type VCS interface {
	// ListAuthorisedRepositories returns a JSON array of repositories, which the user has shared access for
	ListAuthorisedRepositories(ctx echo.Context) error
	HandleWebhookEvents(ctx echo.Context) error
	HandleSetupCallback(ctx echo.Context) error
	// ListHandlers lists all the handler funcs
	ListHandlers() []echo.HandlerFunc
	// RegisterRoutes takes in a echo.Group (aka sub router) which is prefix with VCS name
	// eg: for GitHub, the sub router would be prefixed with "/github", for GitLab "/gitlab"
	RegisterRoutes(subRouter *echo.Group)
	HandleAppFinish(ctx echo.Context) error
	CreateInitialPR(ctx echo.Context) error
}

type VCSStore interface {
	GetUserByID(ctx context.Context, userId uuid.UUID) (*types.User, error)
	UpdateUser(ctx context.Context, u *types.User) (*types.User, error)
}

type Repository struct {
	Owner string
	Name  string
}

type InitialPRRequest struct {
	DockerfilePath string `json:"dockerfile_path"`
	RepositoryName string `json:"repository_name"`
}

const (
	ListAuthorisedRepositoriesEndpoint = "/app/repo/list"
	HandleSetupCallbackEndpoint        = "/app/callback"
	HandleWebhookEventsEndpoint        = "/app/webhooks/listen"
	HandleAppFinishEndpoint            = "/app/setup/finish"
	CreateInitialPREndpoint            = "/app/workflows/create"
)
