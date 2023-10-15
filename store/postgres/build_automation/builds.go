package build_automation_store

import (
	"context"
	"fmt"
	"time"

	gha_v1 "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	"github.com/fatih/color"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

// BulkDeleteBuildJobs implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) BulkDeleteBuildJobs(ctx context.Context, req *gha_v1.BulkDeleteBuildJobsRequest) error {
	query := `delete from build_jobs where id in $1 and owner=$2`

	_, err := p.conn.Exec(ctx, query, req.GetJobIds(), req.GetRepositoryId())
	if err != nil {
		return fmt.Errorf("ERR_BULK_DELETE_JOBS: %w", err)
	}

	return nil
}

// DeleteJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) DeleteJob(ctx context.Context, req *gha_v1.DeleteJobRequest) error {
	query := `delete from build_jobs where id=$1 and owner=$1`

	_, err := p.conn.Exec(ctx, query, req.GetRunId(), req.GetRepositoryId())
	if err != nil {
		return fmt.Errorf("ERR_DELETE_JOB: %w", err)
	}

	return nil
}

// GetBuildJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) GetBuildJob(ctx context.Context, req *gha_v1.GetBuildJobRequest) (*gha_v1.GetBuildJobResponse, error) {
	query := `select 
    id, logs_url, status, triggered_by, duration, branch, commit_hash, triggered_at
    from 
    build_jobs where id=$1`
	row := p.conn.QueryRow(ctx, query, req.GetJobId())

	job := &gha_v1.GetBuildJobResponse{}
	duration := time.Duration(0)
	if err := row.Scan(
		&job.Id,
		&job.LogsUrl,
		&job.Status,
		&job.TriggeredBy,
		&duration,
		&job.Branch,
		&job.CommitHash,
		&job.TriggeredAt,
	); err != nil {
		return nil, fmt.Errorf("ERR_GET_BUILD_JOB: %w", err)
	}
	job.Duration = durationpb.New(time.Second * duration)

	return job, nil
}

// ListBuildJobs implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) ListBuildJobs(
	ctx context.Context,
	req *gha_v1.ListBuildJobsRequest,
) (*gha_v1.ListBuildJobsResponse, error) {
	query := `select 
    id, logs_url, status, triggered_by, duration, branch, commit_hash, triggered_at
    from 
    build_jobs where owner_id=$1`

	rows, err := p.conn.Query(ctx, query, req.GetId())
	if err != nil {
		return nil, fmt.Errorf("ERR_LIST_BUILD_JOBS: %w", err)
	}

	jobs := &gha_v1.ListBuildJobsResponse{}

	for rows.Next() {
		job := &gha_v1.GetBuildJobResponse{}
		duration := time.Duration(0)
		if err := rows.Scan(
			&job.Id,
			&job.LogsUrl,
			&job.Status,
			&job.TriggeredBy,
			&duration,
			&job.Branch,
			&job.CommitHash,
			&job.TriggeredAt,
		); err != nil {
			return nil, fmt.Errorf("ERR_GET_BUILD_JOB: %w", err)
		}

		job.Duration = durationpb.New(time.Second * duration)

		jobs.Jobs = append(jobs.Jobs, job)
	}

	return jobs, nil
}

// StoreJob implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) StoreJob(ctx context.Context, req *gha_v1.StoreJobRequest) error {
	query := `insert into build_jobs 
    (id, logs_url, status, triggered_by, duration, branch, commit_hash, owner_id)
    values($1,$2,$3,$4,$5,$6,$7,$8)
    `
	color.Yellow("timestamp: %s", req.TriggeredAt.AsTime())
	_, err := p.conn.Exec(
		ctx,
		query,
		req.GetId(),
		req.GetLogsUrl(),
		req.GetStatus(),
		req.GetTriggeredBy(),
		req.GetDuration().GetSeconds(),
		req.GetBranch(),
		req.GetCommitHash(),
		// req.GetTriggeredAt().AsTime(),
		req.GetRepositoryId(),
	)
	if err != nil {
		return fmt.Errorf("ERR_STORE_BUILD_JOB: %w", err)
	}

	return nil
}

// TriggerBuild implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) TriggerBuild(ctx context.Context, req *gha_v1.TriggerBuildRequest) error {
	panic("unimplemented")
}

// CancelBuild implements gha_v1connect.GithubActionsBuildServiceHandler
func (p *pg) CancelBuild(ctx context.Context, req *gha_v1.CancelBuildRequest) error {
	panic("unimplemented")
}
