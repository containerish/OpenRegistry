package automation

import (
	"context"
	"fmt"
	"time"

	common_v1 "github.com/containerish/OpenRegistry/common/v1"
	gha_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BulkDeleteBuildJobs implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) BulkDeleteBuildJobs(ctx context.Context, req *gha_v1.BulkDeleteBuildJobsRequest) error {
	_, err := s.db.NewDelete().Model(&types.RepositoryBuild{}).Where("id in (?)", bun.In(req.GetJobIds())).Exec(ctx)
	if err != nil {
		return fmt.Errorf("ERR_BULK_DELETE_JOBS: %w", err)
	}

	return nil
}

// DeleteJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) DeleteJob(ctx context.Context, req *gha_v1.DeleteJobRequest) error {
	_, err := s.db.NewDelete().Model(&types.RepositoryBuild{}).Where("id = ?", req.GetRunId()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("ERR_DELETE_JOB: %w", err)
	}

	return nil
}

// GetBuildJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) GetBuildJob(ctx context.Context, req *gha_v1.GetBuildJobRequest) (*gha_v1.GetBuildJobResponse, error) {
	var job types.RepositoryBuild
	if err := s.db.NewSelect().Model(&job).Where("id = ?", req.GetJobId()).Scan(ctx); err != nil {
		return nil, fmt.Errorf("ERR_GET_BUILD_JOB: %w", err)
	}

	buildJob := &gha_v1.GetBuildJobResponse{
		Id: &common_v1.UUID{
			Value: job.ID.String(),
		},
		LogsUrl:     job.LogsURL,
		Status:      job.Status,
		TriggeredBy: job.TriggeredBy,
		Duration:    durationpb.New(job.Duration),
		Branch:      job.Branch,
		CommitHash:  job.CommitHash,
		TriggeredAt: timestamppb.New(job.TriggeredAt),
		RepositoryId: &common_v1.UUID{
			Value: job.RepositoryID.String(),
		},
	}

	return buildJob, nil
}

// ListBuildJobs implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) ListBuildJobs(
	ctx context.Context,
	req *gha_v1.ListBuildJobsRequest,
) (*gha_v1.ListBuildJobsResponse, error) {
	jobs := make([]*types.RepositoryBuild, 0)
	if err := s.db.NewSelect().Model(jobs).Where("repository_id = ?", req.GetRepositoryId()).Scan(ctx); err != nil {
		return nil, fmt.Errorf("ERR_LIST_BUILD_JOBS: %w", err)
	}

	protoJobs := &gha_v1.ListBuildJobsResponse{
		Jobs: make([]*gha_v1.GetBuildJobResponse, len(jobs)),
	}

	for i, job := range jobs {
		protoJobs.Jobs[i] = &gha_v1.GetBuildJobResponse{
			Id: &common_v1.UUID{
				Value: job.ID.String(),
			},
			LogsUrl:     job.LogsURL,
			Status:      job.Status,
			TriggeredBy: job.TriggeredBy,
			Duration:    durationpb.New(job.Duration),
			Branch:      job.Branch,
			CommitHash:  job.CommitHash,
			TriggeredAt: timestamppb.New(job.TriggeredAt),
			RepositoryId: &common_v1.UUID{
				Value: job.RepositoryID.String(),
			},
		}
	}

	return protoJobs, nil
}

// StoreJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) StoreJob(ctx context.Context, req *gha_v1.StoreJobRequest) error {
	job := &types.RepositoryBuild{
		TriggeredAt:  req.GetTriggeredAt().AsTime(),
		UpdatedAt:    time.Time{},
		CreatedAt:    time.Now(),
		LogsURL:      req.GetLogsUrl(),
		Status:       req.GetStatus(),
		TriggeredBy:  req.GetTriggeredBy(),
		Branch:       req.GetBranch(),
		CommitHash:   req.GetCommitHash(),
		Duration:     req.GetDuration().AsDuration(),
		RepositoryID: uuid.MustParse(req.GetRepositoryId().String()),
		ID:           uuid.New(),
	}

	if _, err := s.db.NewInsert().Model(job).Exec(ctx); err != nil {
		return fmt.Errorf("ERR_STORE_BUILD_JOB: %w", err)
	}

	return nil
}

// TriggerBuild implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) TriggerBuild(ctx context.Context, req *gha_v1.TriggerBuildRequest) error {
	panic("unimplemented")
}

// CancelBuild implements gha_v1connect.GithubActionsBuildServiceHandler
func (s *store) CancelBuild(ctx context.Context, req *gha_v1.CancelBuildRequest) error {
	panic("unimplemented")
}
