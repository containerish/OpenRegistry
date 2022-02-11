package router

import (
	"net/http"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/labstack/echo/v4"
)

// These are helper functions to Register depending on the usability

// RegisterAuthRoutes includes all the auth related endpoints
func RegisterAuthRoutes(authRouter *echo.Group, authSvc auth.Authentication) {

	authRouter.Add(http.MethodPost, "/signup", authSvc.SignUp)
	authRouter.Add(http.MethodPost, "/signin", authSvc.SignIn)
	authRouter.Add(http.MethodPost, "/token", authSvc.SignIn)

}

// RegisterBetaRoutes contains the experimental features, the betas
func RegisterBetaRoutes(betaRouter *echo.Group, localCache cache.Store) {

	betaRouter.Add(http.MethodPost, "/register", localCache.RegisterForBeta)
	betaRouter.Add(http.MethodGet, "/register", localCache.GetAllEmail)
}

// InternalRoutes contains the routes to be kept  limited and not be exposed to user
func InternalRoutes(internal *echo.Group, persistentStore postgres.PersistentStore) {

	internal.Add(http.MethodGet, "/metadata", persistentStore.Metadata)
	internal.Add(http.MethodGet, "/digests", persistentStore.LayerDigests)
}
