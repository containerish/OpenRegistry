package build_automation_store

import (
	"context"
	"time"

	"github.com/containerish/OpenRegistry/config"
	gha_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/fatih/color"
	"github.com/jackc/pgx/v4/pgxpool"
)

type pg struct {
	conn *pgxpool.Pool
}

type BuildAutomationStore interface {
	StoreProject(context.Context, *gha_v1.CreateProjectRequest) error
	GetProject(context.Context, *gha_v1.GetProjectRequest) (*gha_v1.GetProjectResponse, error)
	DeleteProject(context.Context, *gha_v1.DeleteProjectRequest) error
	ListProjects(context.Context, *gha_v1.ListProjectsRequest) (*gha_v1.ListProjectsResponse, error)
	CancelBuild(context.Context, *gha_v1.CancelBuildRequest) error
	TriggerBuild(context.Context, *gha_v1.TriggerBuildRequest) error
	StoreJob(context.Context, *gha_v1.StoreJobRequest) error
	ListBuildJobs(context.Context, *gha_v1.ListBuildJobsRequest) (*gha_v1.ListBuildJobsResponse, error)
	GetBuildJob(context.Context, *gha_v1.GetBuildJobRequest) (*gha_v1.GetBuildJobResponse, error)
	BulkDeleteBuildJobs(context.Context, *gha_v1.BulkDeleteBuildJobsRequest) error
	DeleteJob(context.Context, *gha_v1.DeleteJobRequest) error
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

	color.Green("Service - BuildAutomationStore - connection to database successful")
	return &pg{conn: conn}, nil
}
