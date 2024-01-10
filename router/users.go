package router

import (
	"net/http"

	"github.com/containerish/OpenRegistry/api/users"
	"github.com/labstack/echo/v4"
)

func RegisterUserRoutes(router *echo.Group, api users.UserApi, middlewares ...echo.MiddlewareFunc) {
	router.Add(http.MethodGet, "/search", api.SearchUsers, middlewares...)
	router.Add(http.MethodPost, "/token", api.CreateUserToken, middlewares...)
	router.Add(http.MethodGet, "/token", api.ListUserToken, middlewares...)
}
