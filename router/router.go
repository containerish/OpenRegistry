package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/api/users"
	"github.com/containerish/OpenRegistry/auth"
	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/orgmode"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/containerish/OpenRegistry/store/v1/automation"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
	users_store "github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Register is the entry point that registers all the endpoints
// nolint
func Register(
	cfg *config.OpenRegistryConfig,
	logger telemetry.Logger,
	registryApi registry.Registry,
	authApi auth.Authentication,
	webauthnApi auth_server.WebauthnServer,
	extensionsApi extensions.Extenion,
	orgModeApi orgmode.OrgMode,
	usersApi users.UserApi,
	healthCheckApi http.HandlerFunc,
	registryStore registry_store.RegistryStore,
	usersStore users_store.UserStore,
	automationStore automation.BuildAutomationStore,
	dfs dfs.DFS,
) *echo.Echo {
	e := setDefaultEchoOptions(cfg.WebAppConfig, healthCheckApi)

	baseAPIRouter := e.Group("/api")
	githubRouter := e.Group("/github")
	authRouter := e.Group(Auth)
	webauthnRouter := e.Group(Webauthn)
	orgModeRouter := baseAPIRouter.Group("/org", authApi.JWTRest(), orgModeApi.AllowOrgAdmin())
	ociRouter := e.Group(V2, registryNamespaceValidator(logger), authApi.BasicAuth(), authApi.JWT())
	userApiRouter := baseAPIRouter.Group("/users", authApi.JWTRest())
	nsRouter := ociRouter.Group(Namespace, authApi.RepositoryPermissionsMiddleware())
	authGithubRouter := authRouter.Group(GitHub)

	ociRouter.Add(http.MethodGet, Root, registryApi.ApiVersion)
	e.Add(http.MethodGet, TokenAuth, authApi.Token, authApi.RepositoryPermissionsMiddleware())
	authGithubRouter.Add(http.MethodGet, "/callback", authApi.GithubLoginCallbackHandler)
	authGithubRouter.Add(http.MethodGet, "/login", authApi.LoginWithGithub)

	RegisterUserRoutes(userApiRouter, usersApi)
	RegisterNSRoutes(nsRouter, registryApi, registryStore, logger)
	RegisterAuthRoutes(authRouter, authApi)
	RegisterExtensionsRoutes(ociRouter, registryApi, extensionsApi, authApi.JWTRest())
	RegisterWebauthnRoutes(webauthnRouter, webauthnApi)
	RegisterOrgModeRoutes(orgModeRouter, orgModeApi)
	RegisterVulnScaningRoutes(
		usersStore,
		cfg.Integrations.GetClairConfig(),
		&cfg.Registry.Auth,
		logger,
		registryStore.GetLayersLinksForManifest,
		dfs.GeneratePresignedURL,
		cfg.WebAppConfig.AllowedEndpoints,
	)

	if cfg.Integrations.GetGithubConfig() != nil && cfg.Integrations.GetGithubConfig().Enabled {
		RegisterGitHubRoutes(
			githubRouter,
			cfg.Integrations.GetGithubConfig(),
			cfg.Environment,
			&cfg.Registry.Auth,
			logger,
			cfg.WebAppConfig.AllowedEndpoints,
			usersStore,
			automationStore,
			cfg.WebAppConfig.AllowedEndpoints,
			cfg.Endpoint(),
		)
	}

	//catch-all will redirect user back to the web interface
	e.Add(http.MethodGet, "", func(ctx echo.Context) error {
		webAppURL := ""
		for _, url := range cfg.WebAppConfig.AllowedEndpoints {
			if url == ctx.Request().Header.Get("Origin") {
				webAppURL = url
				break
			}
		}

		if strings.HasSuffix(ctx.Request().Header.Get("Origin"), "openregistry-web.pages.dev") {
			webAppURL = ctx.Request().Header.Get("Origin")
		}

		if webAppURL != "" {
			echoErr := ctx.Redirect(http.StatusTemporaryRedirect, webAppURL)
			logger.Log(ctx, nil).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"API": "running",
		})

		logger.Log(ctx, nil).Send()
		return echoErr
	})

	return e
}

func setDefaultEchoOptions(webConfig config.WebAppConfig, healthCheck http.HandlerFunc) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     webConfig.AllowedEndpoints,
		AllowMethods:     middleware.DefaultCORSConfig.AllowMethods,
		AllowHeaders:     middleware.DefaultCORSConfig.AllowHeaders,
		AllowCredentials: true,
		ExposeHeaders:    middleware.DefaultCORSConfig.ExposeHeaders,
		MaxAge:           750,
	}))

	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		Generator: func() string {
			requestId, err := uuid.NewRandom()
			if err != nil {
				return time.Now().Format(time.RFC3339Nano)
			}
			return requestId.String()
		},
		TargetHeader: echo.HeaderXRequestID,
	}))
	p := prometheus.NewPrometheus("OpenRegistry", nil)
	p.Use(e)

	e.Add(http.MethodGet, "/health", echo.WrapHandler(healthCheck))

	return e
}
