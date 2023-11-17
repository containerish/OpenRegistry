package automation

import (
	"context"

	gha_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"github.com/uptrace/bun"
)

type store struct {
	logger telemetry.Logger
	db     *bun.DB
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

func New(db *bun.DB, logger telemetry.Logger) (BuildAutomationStore, error) {

	color.Green("Service - BuildAutomationStore - connection to database successful")
	return &store{db: db, logger: logger}, nil
}

func (s *store) Close() {
	_ = s.db.Close()
}
