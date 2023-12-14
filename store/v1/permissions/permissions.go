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
		// GetUserPermissionsForNamespace returns permissions for the given namespace for the user.
		// It doesn't return any errors. If the user has permissions, they're be reflect in the returned
		// *types.Permissions struct, otherwise, the returned type will be an empty, non-nil struct
		GetUserPermissionsForNamespace(ctx context.Context, ns string, userID uuid.UUID) *types.Permissions
		AddPermissions(ctx context.Context, perm *types.AddUsersToOrgRequest) error
		UpdatePermissions(ctx context.Context, perm *types.Permissions) error
		RemoveUserFromOrg(ctx context.Context, orgID, userID uuid.UUID) error
	}

	permissionStore struct {
		logger telemetry.Logger
		db     *bun.DB
	}
)

func New(bunWrappedDB *bun.DB, logger telemetry.Logger) PermissionsStore {
	store := &permissionStore{
		db:     bunWrappedDB,
		logger: logger,
	}

	return store
}
