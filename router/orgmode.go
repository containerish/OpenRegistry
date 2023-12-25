package router

import (
	"github.com/containerish/OpenRegistry/orgmode"
	"github.com/labstack/echo/v4"
)

func RegisterOrgModeRoutes(router *echo.Group, svc orgmode.OrgMode, mws ...echo.MiddlewareFunc) {
	router.POST("/migrate", svc.MigrateToOrg, mws...)
	router.POST("/users", svc.AddUserToOrg, mws...)
	router.PATCH("/permissions/users", svc.UpdateUserPermissions, mws...)
	router.DELETE("/permissions/users/:orgId/:userId", svc.RemoveUserFromOrg, mws...)
}
