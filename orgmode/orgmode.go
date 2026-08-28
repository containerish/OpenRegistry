package orgmode

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/permissions"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type (
	OrgMode interface {
		MigrateToOrg(ctx echo.Context) error
		AddUserToOrg(ctx echo.Context) error
		RemoveUserFromOrg(ctx echo.Context) error
		UpdateUserPermissions(ctx echo.Context) error
		GetOrgUsers(ctx echo.Context) error
		AllowOrgAdmin() echo.MiddlewareFunc
	}

	orgMode struct {
		logger           telemetry.Logger
		permissionsStore permissions.PermissionsStore
		userStore        users.UserStore
	}
)

func New(permStore permissions.PermissionsStore, usersStore users.UserStore, logger telemetry.Logger) OrgMode {
	return &orgMode{
		permissionsStore: permStore,
		userStore:        usersStore,
		logger:           logger,
	}
}

// MigrateToOrg implements OrgMode.
func (o *orgMode) MigrateToOrg(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*types.MigrateToOrgRequest)
	if err := o.userStore.ConvertUserToOrg(ctx.Request().Context(), body.UserID); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "user account converted to organization successfully",
	})
	o.logger.Log(ctx, nil).Send()
	return echoErr
}

// AddUserToOrg implements OrgMode.
func (o *orgMode) AddUserToOrg(ctx echo.Context) error {
	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*types.AddUsersToOrgRequest)

	userIdsToCheck := make([]uuid.UUID, len(body.Users))
	for i, u := range body.Users {
		userIdsToCheck[i] = u.ID
	}

	ok := o.userStore.MatchUserType(ctx.Request().Context(), types.UserTypeRegular, userIdsToCheck...)
	if !ok {
		err := fmt.Errorf("invalid user ids in the request")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	if err := o.permissionsStore.AddPermissions(ctx.Request().Context(), body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "user added to organization successfully",
	})
	o.logger.Log(ctx, nil).Send()
	return echoErr
}

// RemoveUserFromOrg implements OrgMode.
func (o *orgMode) RemoveUserFromOrg(ctx echo.Context) error {
	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*types.RemoveUserFromOrgRequest)

	if err := o.permissionsStore.RemoveUserFromOrg(ctx.Request().Context(), body.OrganizationID, body.UserID); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "user removed from organization successfully",
	})
	o.logger.Log(ctx, nil).Send()
	return echoErr
}

// UpdateUserPermissions implements OrgMode.
func (o *orgMode) UpdateUserPermissions(ctx echo.Context) error {
	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*types.Permissions)
	if err := o.permissionsStore.UpdatePermissions(ctx.Request().Context(), body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "user permissions updated successfully",
	})
	o.logger.Log(ctx, nil).Send()

	return echoErr
}

func (o *orgMode) GetOrgUsers(ctx echo.Context) error {
	orgID, err := uuid.Parse(ctx.QueryParam("org_id"))
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	orgMembers, err := o.userStore.GetOrgUsersByOrgID(ctx.Request().Context(), orgID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	if len(orgMembers) == 0 {
		orgMembers = make([]*types.Permissions, 0)
	}

	echoErr := ctx.JSON(http.StatusOK, orgMembers)
	o.logger.Log(ctx, nil).Send()
	return echoErr
}
