package router

import (
	"net/http"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/labstack/echo/v4"
)

// These are helper functions to Register depending on the usability
// RegisterAuthRoutes includes all the auth related endpoints
func RegisterAuthRoutes(authRouter *echo.Group, authSvc auth.Authentication) {
	authRouter.Add(http.MethodPost, "/signup", authSvc.SignUp)
	authRouter.Add(http.MethodPost, "/send-email/welcome", authSvc.Invites)
	authRouter.Add(http.MethodGet, "/signup/verify", authSvc.VerifyEmail)
	authRouter.Add(http.MethodPost, "/signin", authSvc.SignIn)
	authRouter.Add(http.MethodPost, "/token", authSvc.SignIn)
	authRouter.Add(http.MethodDelete, "/signout", authSvc.SignOut)
	authRouter.Add(http.MethodGet, "/sessions/me", authSvc.ReadUserWithSession)
	authRouter.Add(http.MethodDelete, "/sessions", authSvc.ExpireSessions)
	authRouter.Add(http.MethodGet, "/renew", authSvc.RenewAccessToken)
	authRouter.Add(http.MethodPost, "/reset-password", authSvc.ResetPassword, authSvc.JWTRest())
	authRouter.Add(http.MethodPost, "/reset-forgotten-password", authSvc.ResetForgottenPassword, authSvc.JWT())
	authRouter.Add(http.MethodGet, "/forgot-password", authSvc.ForgotPassword)
}
