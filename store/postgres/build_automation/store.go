package build_automation_store

import (
	"context"
	"time"

	"github.com/containerish/OpenRegistry/config"
	github_actions_v1 "github.com/containerish/OpenRegistry/services/kone/github_actions/v1"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v4/pgxpool"
)

type pg struct {
	conn *pgxpool.Pool
}

type BuildAutomationStore interface {
	StoreProject(ctx context.Context, project *github_actions_v1.CreateProjectRequest) error
	GetProject(ctx context.Context, project *github_actions_v1.GetProjectRequest) (*github_actions_v1.GetProjectResponse, error)
	DeleteProject(ctx context.Context, project *github_actions_v1.DeleteProjectRequest) error
	ListProjects(ctx context.Context, project *github_actions_v1.ListProjectsRequest) (*github_actions_v1.ListProjectsResponse, error)
	Close()
}

func New(cfg *config.Store) (BuildAutomationStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	pgxCofig, err := pgxpool.ParseConfig(cfg.Endpoint())
	if err != nil {
		return nil, err
	}

	conn, err := pgxpool.ConnectConfig(ctx, pgxCofig)
	if err != nil {
		return nil, err
	}

	color.Green("connection to database successful")
	return &pg{conn: conn}, nil
}
