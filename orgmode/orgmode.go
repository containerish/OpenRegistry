package orgmode

import (
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
		AllowOrgAdmin() echo.MiddlewareFunc
	}

	MigrateToOrgRequest struct {
		UserID uuid.UUID `json:"user_id"`
	}

	RemoveUserFromOrgRequest struct {
		UserID         uuid.UUID `json:"user_id"`
		OrganizationID uuid.UUID `json:"organization_id"`
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

	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*MigrateToOrgRequest)
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
	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*types.Permissions)

	user, err := o.userStore.GetUserByID(ctx.Request().Context(), body.UserID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		o.logger.Log(ctx, err).Send()
		return echoErr
	}

	if user.UserType != types.UserTypeRegular.String() {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "only regular users can be added to an organization",
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
	body := ctx.Get(string(types.OrgModeRequestBodyContextKey)).(*RemoveUserFromOrgRequest)

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
