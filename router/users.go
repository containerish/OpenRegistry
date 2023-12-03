package router

import (
	"net/http"

	"github.com/containerish/OpenRegistry/api/users"
	"github.com/labstack/echo/v4"
)

func RegisterUserRoutes(router *echo.Group, api users.UserApi) {
	router.Add(http.MethodGet, "/search", api.SearchUsers)
}
