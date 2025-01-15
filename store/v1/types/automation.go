package types

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type (
	RepositoryBuild struct {
		bun.BaseModel `bun:"table:repository_builds,alias:b" json:"-"`

		TriggeredAt  time.Time                 `bun:"triggered_at,type:timestamptz" json:"triggered_at"`
		UpdatedAt    time.Time                 `bun:"updated_at" json:"updated_at,omitempty" validate:"-"`
		CreatedAt    time.Time                 `bun:"created_at" json:"created_at,omitempty" validate:"-"`
		Repository   *ContainerImageRepository `bun:"rel:belongs-to,join:repository_id=id" json:"-"`
		LogsURL      string                    `bun:"logs_url" json:"logs_url"`
		Status       string                    `bun:"status" json:"status"`
		TriggeredBy  string                    `bun:"triggered_by" json:"triggered_by"`
		Branch       string                    `bun:"branch" json:"branch"`
		CommitHash   string                    `bun:"commit_hash" json:"commit_hash"`
		Duration     time.Duration             `bun:"duration" json:"duration"`
		RepositoryID uuid.UUID                 `bun:"repository_id,type:uuid" json:"repository_id"`
		ID           uuid.UUID                 `bun:"id,type:uuid,pk" json:"id,omitempty" validate:"-"`
	}

	RepositoryBuildProject struct {
		bun.BaseModel `bun:"table:repository_build_projects,alias:p" json:"-"`

		UpdatedAt            time.Time                 `bun:"updated_at" json:"updated_at,omitempty" validate:"-"`
		CreatedAt            time.Time                 `bun:"created_at" json:"created_at,omitempty" validate:"-"`
		EnvironmentVariables map[string]string         `bun:"environment_variables,type:jsonb" json:"environment_variables"`
		Repository           *ContainerImageRepository `bun:"rel:belongs-to,join:repository_id=id" json:"-"`
		User                 *User                     `bun:"rel:belongs-to,join:repository_owner_id=id" json:"-"`
		Name                 string                    `bun:"name" json:"name"`
		ProductionBranch     string                    `bun:"production_branch" json:"production_branch"`
		BuildTool            string                    `bun:"build_tool" json:"build_tool"`
		ExecCommand          string                    `bun:"exec_command" json:"exec_command"`
		WorkflowFile         string                    `bun:"workflow_file" json:"workflow_file"`
		ID                   uuid.UUID                 `bun:"id,type:uuid,pk" json:"id,omitempty" validate:"-"`
		RepositoryID         uuid.UUID                 `bun:"repository_id,type:uuid" json:"repository_id"`
		RepositoryOwnerID    uuid.UUID                 `bun:"repository_owner_id,type:uuid" json:"repository_owner_id"`
	}
)
