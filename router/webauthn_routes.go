package router

import (
	"net/http"

	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/labstack/echo/v4"
)

func RegisterWebauthnRoutes(
	router *echo.Group,
	webauthnServer auth_server.WebauthnServer,
) {
	router.Add(http.MethodPost, "/registration/begin", webauthnServer.BeginRegistration)
	router.Add(http.MethodDelete, "/registration/rollback", webauthnServer.RollbackRegistration)
	router.Add(http.MethodPost, "/registration/finish", webauthnServer.FinishRegistration)
	router.Add(http.MethodGet, "/login/begin", webauthnServer.BeginLogin)
	router.Add(http.MethodPost, "/login/finish", webauthnServer.FinishLogin)
}
