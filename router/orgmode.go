package router

import (
	"github.com/containerish/OpenRegistry/orgmode"
	"github.com/labstack/echo/v4"
)

func RegisterOrgModeRoutes(router *echo.Group, svc orgmode.OrgMode) {
	router.POST("/migrate", svc.MigrateToOrg)
	router.POST("/users", svc.AddUserToOrg)
	router.GET("/users", svc.GetOrgUsers)
	router.PATCH("/permissions/users", svc.UpdateUserPermissions)
	router.DELETE("/permissions/users/:orgId/:userId", svc.RemoveUserFromOrg)
}
