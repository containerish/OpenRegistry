package orgmode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (o *orgMode) AllowOrgAdmin() echo.MiddlewareFunc {
	return func(handler echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			p := ctx.Request().URL.Path
			switch {
			case p == "/org/migrate":
				var body MigrateToOrgRequest
				if err := json.NewDecoder(ctx.Request().Body).Decode(&body); err != nil {
					echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
						"error": err.Error(),
					})
					o.logger.Log(ctx, err).Send()
					return echoErr
				}
				ctx.Set(string(types.OrgModeRequestBodyContextKey), &body)
				return handler(ctx)
			case p == "/org/users" || strings.HasPrefix(p, "/org/permissions/users"):
				user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
				if !ok {
					errMsg := fmt.Errorf("missing authentication information")
					echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
						"error": errMsg.Error(),
					})
					o.logger.Log(ctx, errMsg).Send()
					return echoErr
				}

				orgAdmin, err := o.userStore.GetOrgAdmin(ctx.Request().Context(), user.ID)
				if err != nil {
					echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
						"error": err.Error(),
					})
					o.logger.Log(ctx, err).Send()
					return echoErr
				}
				switch ctx.Request().Method {
				case http.MethodPost, http.MethodPatch:
					var perms types.Permissions
					if err = json.NewDecoder(ctx.Request().Body).Decode(&perms); err != nil {
						echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
							"error": err.Error(),
						})
						o.logger.Log(ctx, err).Send()
						return echoErr
					}
					defer ctx.Request().Body.Close()
					// @TODO(jay-dee7) - Use a better comparison method
					if orgAdmin.ID.String() != perms.OrganizationID.String() {
						err = fmt.Errorf("action not allowed")
						echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
							"error": err.Error(),
						})
						o.logger.Log(ctx, err).Send()
						return echoErr
					}
					ctx.Set(string(types.OrgModeRequestBodyContextKey), &perms)
				case http.MethodDelete:
					if strings.HasPrefix(p, fmt.Sprintf("/org/permissions/users/%s/", orgAdmin.ID.String())) {
						orgID, err := uuid.Parse(ctx.Param("orgId"))
						if err != nil {
							echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
								"error": err.Error(),
							})
							o.logger.Log(ctx, err).Send()
							return echoErr
						}
						userID, err := uuid.Parse(ctx.Param("userId"))
						if err != nil {
							echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
								"error": err.Error(),
							})
							o.logger.Log(ctx, err).Send()
							return echoErr
						}

						body := &RemoveUserFromOrgRequest{
							UserID:         userID,
							OrganizationID: orgID,
						}

						if body.OrganizationID.String() != orgAdmin.ID.String() {
							err = fmt.Errorf("action not allowed")
							echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
								"error": err.Error(),
							})
							o.logger.Log(ctx, err).Send()
							return echoErr
						}
						ctx.Set(string(types.OrgModeRequestBodyContextKey), body)
					}
				}
			}

			return handler(ctx)
		}
	}
}
