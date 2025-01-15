package orgmode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (o *orgMode) AllowOrgAdmin() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
			if !ok {
				err := fmt.Errorf("missing authentication information")
				echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
					"error": err.Error(),
				})
				o.logger.Log(ctx, err).Send()
				return echoErr
			}

			p := ctx.Request().URL.Path
			switch {
			case p == "/api/org/migrate":
				var body types.MigrateToOrgRequest
				if err := json.NewDecoder(ctx.Request().Body).Decode(&body); err != nil {
					echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
						"error": err.Error(),
					})
					o.logger.Log(ctx, err).Send()
					return echoErr
				}
				defer ctx.Request().Body.Close()

				// only allow self-migrate
				if !strings.EqualFold(user.ID.String(), body.UserID.String()) {
					err := fmt.Errorf("access not allowed")
					echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
						"error": err.Error(),
					})
					o.logger.Log(ctx, err).Send()
					return echoErr
				}

				ctx.Set(string(types.OrgModeRequestBodyContextKey), &body)
				return handler(ctx)
			case p == "/api/org/users" || strings.HasPrefix(p, "/api/org/permissions/users"):
				orgOwner, err := o.userStore.GetOrgAdmin(ctx.Request().Context(), user.ID)
				if err != nil {
					echoErr := ctx.JSON(http.StatusForbidden, echo.Map{
						"error":   err.Error(),
						"message": "user does not have permission to add users to organization",
					})
					o.logger.Log(ctx, err).Send()
					return echoErr
				}

				switch ctx.Request().Method {
				case http.MethodPost:
					if err = o.parseAddUsersToOrgRequest(ctx, user, orgOwner); err != nil {
						echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
							"error": err.Error(),
						})
						o.logger.Log(ctx, err).Send()
						return echoErr
					}
				case http.MethodPatch:
					if err = o.handleOrgModePermissionRequests(ctx, user, orgOwner); err != nil {
						echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
							"error": err.Error(),
						})
						o.logger.Log(ctx, err).Send()
						return echoErr
					}
				case http.MethodDelete:
					if err = o.handleOrgModeRemoveUser(ctx, p, orgOwner); err != nil {
						echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
							"error": err.Error(),
						})
						o.logger.Log(ctx, err).Send()
						return echoErr
					}
				}
			}

			return handler(ctx)
		}
	}
}

func (o *orgMode) parseAddUsersToOrgRequest(ctx echo.Context, user *types.User, orgOwner *types.User) error {
	var body types.AddUsersToOrgRequest
	if err := ctx.Bind(&body); err != nil {
		return err
	}
	defer ctx.Request().Body.Close()

	// @TODO(jay-dee7) - Use a better comparison method
	if !strings.EqualFold(orgOwner.ID.String(), body.OrganizationID.String()) {
		return fmt.Errorf("organization id mismatch, invalid organization id")
	}

	parsedBody := types.AddUsersToOrgRequest{
		OrganizationID: body.OrganizationID,
	}
	for _, perm := range body.Users {
		if perm.Pull && perm.Push && !perm.IsAdmin {
			perm.IsAdmin = true
		}

		if perm.IsAdmin {
			perm.Pull = true
			perm.Push = true
		}

		parsedBody.Users = append(parsedBody.Users, perm)
	}

	ctx.Set(string(types.OrgModeRequestBodyContextKey), &parsedBody)
	return nil
}

func (o *orgMode) handleOrgModePermissionRequests(ctx echo.Context, user *types.User, orgOwner *types.User) error {
	var perms types.Permissions
	if err := json.NewDecoder(ctx.Request().Body).Decode(&perms); err != nil {
		return err
	}
	defer ctx.Request().Body.Close()

	// @TODO(jay-dee7) - Use a better comparison method
	if !strings.EqualFold(orgOwner.ID.String(), perms.OrganizationID.String()) {
		return fmt.Errorf("no organization exists with id: %s", perms.OrganizationID.String())
	}

	// userid and org id cannot be the same
	if strings.EqualFold(perms.UserID.String(), perms.OrganizationID.String()) {
		return fmt.Errorf("user id and organization id can not be the same")
	}

	if perms.Pull && perms.Push && !perms.IsAdmin {
		perms.IsAdmin = true
	}

	if perms.IsAdmin {
		perms.Pull = true
		perms.Push = true
	}

	ctx.Set(string(types.OrgModeRequestBodyContextKey), &perms)
	return nil
}

func (o *orgMode) handleOrgModeRemoveUser(ctx echo.Context, p string, orgOwner *types.User) error {
	if strings.HasPrefix(p, fmt.Sprintf("/api/org/permissions/users/%s/", orgOwner.ID.String())) {
		orgID, err := uuid.Parse(ctx.Param("orgId"))
		if err != nil {
			return err
		}
		userID, err := uuid.Parse(ctx.Param("userId"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			o.logger.Log(ctx, err).Send()
			return echoErr
		}

		body := &types.RemoveUserFromOrgRequest{
			UserID:         userID,
			OrganizationID: orgID,
		}

		if body.OrganizationID.String() != orgOwner.ID.String() {
			err = fmt.Errorf("action not allowed")
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			o.logger.Log(ctx, err).Send()
			return echoErr
		}
		ctx.Set(string(types.OrgModeRequestBodyContextKey), body)
	}

	return nil
}
