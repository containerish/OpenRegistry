package permissions

import (
	"context"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type (
	PermissionsStore interface {
		GetAllUserPermissions(ctx context.Context, userID uuid.UUID) ([]*types.Permissions, error)
		GetUserPermissionsForOrg(ctx context.Context, orgID, userID uuid.UUID) (*types.Permissions, error)
		AddPermissions(ctx context.Context, perm *types.Permissions) error
		UpdatePermissions(ctx context.Context, perm *types.Permissions) error
		RemoveUserFromOrg(ctx context.Context, orgID, userID uuid.UUID) error
	}

	permissionStore struct {
		logger telemetry.Logger
		db     *bun.DB
	}
)

func NewStore(bunWrappedDB *bun.DB, logger telemetry.Logger) PermissionsStore {
	store := &permissionStore{
		db:     bunWrappedDB,
		logger: logger,
	}

	return store
}
