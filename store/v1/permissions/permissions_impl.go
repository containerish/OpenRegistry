package permissions

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// AddPermission implements PermissionsStore.
func (p *permissionStore) AddPermissions(
	ctx context.Context,
	input *types.AddUsersToOrgRequest,
) error {
	err := p.validateAddOrgMembersInput(input)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationWrite)
	}
	perms := make([]*types.Permissions, len(input.Users))

	for i, p := range input.Users {
		perm := &types.Permissions{
			UserID:         p.ID,
			OrganizationID: input.OrganizationID,
			Push:           p.Push,
			Pull:           p.Pull,
			IsAdmin:        p.IsAdmin,
		}
		if p.IsAdmin {
			perm.Pull = true
			perm.Push = true
		}

		perms[i] = perm
	}

	if _, err = p.db.NewInsert().Model(&perms).Exec(ctx); err != nil {
		return err
	}

	return nil
}

// GetAllUserPermissions implements PermissionsStore.
func (p *permissionStore) GetAllUserPermissions(
	ctx context.Context,
	userID uuid.UUID,
) ([]*types.Permissions, error) {
	var permSet []*types.Permissions

	if err := p.db.NewSelect().Model(&permSet).Where("user_id = ?", userID.String()).Scan(ctx); err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return permSet, nil
}

// GetUserPermissionsForOrg implements PermissionsStore.
func (p *permissionStore) GetUserPermissionsForOrg(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
) (*types.Permissions, error) {
	perm := &types.Permissions{}

	err := p.
		db.
		NewSelect().
		Model(perm).
		Where("user_id = ?", userID.String()).
		Where("organization_id = ?", orgID).
		Scan(ctx)

	if err != nil {
		return nil, v1.WrapDatabaseError(err, v1.DatabaseOperationRead)
	}

	return perm, nil
}

func (p *permissionStore) UpdatePermissions(ctx context.Context, perm *types.Permissions) error {
	err := p.validatePermissionInput(perm)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	_, err = p.
		db.
		NewUpdate().
		Model(perm).
		Where("organization_id = ?", perm.OrganizationID).
		Where("user_id = ?", perm.UserID).
		Exec(ctx)
	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationUpdate)
	}

	return nil
}

func (p *permissionStore) validatePermissionInput(perm *types.Permissions) error {
	if perm == nil {
		return fmt.Errorf("permission set is nil")
	}

	if perm.UserID.String() == "" {
		return fmt.Errorf("invalid user id")
	}

	if perm.OrganizationID.String() == "" {
		return fmt.Errorf("invalid organization id")
	}

	return nil
}

func (p *permissionStore) validateAddOrgMembersInput(input *types.AddUsersToOrgRequest) error {
	if input == nil {
		return fmt.Errorf("permission set is nil")
	}

	if input.OrganizationID.String() == "" {
		return fmt.Errorf("invalid organization id")
	}

	for _, u := range input.Users {
		if u.ID.String() == "" {
			return fmt.Errorf("invalid user id")
		}
	}

	return nil
}

func (p *permissionStore) RemoveUserFromOrg(ctx context.Context, orgID, userID uuid.UUID) error {
	perm := &types.Permissions{}
	_, err := p.
		db.
		NewDelete().
		Model(perm).
		Where("organization_id = ?", orgID.String()).
		Where("user_id = ?", userID.String()).
		Exec(ctx)

	if err != nil {
		return v1.WrapDatabaseError(err, v1.DatabaseOperationDelete)
	}

	return nil
}
func (p *permissionStore) GetUserPermissionsForNamespace(
	ctx context.Context,
	ns string,
	userID uuid.UUID,
) *types.Permissions {
	perms := &types.Permissions{}

	nsParts := strings.Split(ns, "/")
	if len(nsParts) != 2 {
		return perms
	}
	orgName := nsParts[0]

	q := p.
		db.
		NewSelect().
		Model(perms).
		Relation("Organization", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.
				Where(`"organization"."username" = ?`, orgName).
				Where(`"organization"."user_type" = ?`, types.UserTypeOrganization.String())
		}).
		Relation("User", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.
				Where(`"user"."user_type" = ?`, types.UserTypeRegular.String()).
				Where(`"user"."id" = ?`, userID)
		})

	if err := q.Scan(ctx); err != nil {
		return perms
	}

	return perms

}
