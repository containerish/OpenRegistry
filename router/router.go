package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/api/users"
	"github.com/containerish/OpenRegistry/auth"
	auth_server "github.com/containerish/OpenRegistry/auth/server"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/orgmode"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
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
	e *echo.Echo,
	reg registry.Registry,
	authSvc auth.Authentication,
	webauthnServer auth_server.WebauthnServer,
	ext extensions.Extenion,
	registryStore registry_store.RegistryStore,
	orgModeSvc orgmode.OrgMode,
	userApi users.UserApi,
	logger telemetry.Logger,
) {
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.WebAppConfig.AllowedEndpoints,
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

	e.Add(http.MethodGet, TokenAuth, authSvc.Token, authSvc.RepositoryPermissionsMiddleware())

	p := prometheus.NewPrometheus("OpenRegistry", nil)
	p.Use(e)

	baseAPIRouter := e.Group("/api")
	userApiRouter := baseAPIRouter.Group("/users")

	v2Router := e.Group(V2, registryNamespaceValidator(logger), authSvc.BasicAuth(), authSvc.JWT())
	nsRouter := v2Router.Group(
		Namespace,
		authSvc.ACL(),
		authSvc.RepositoryPermissionsMiddleware(),
	)

	authRouter := e.Group(Auth)
	webauthnRouter := e.Group(Webauthn)
	authGithubRouter := authRouter.Group(GitHub)

	v2Router.Add(http.MethodGet, Root, reg.ApiVersion)

	authGithubRouter.Add(http.MethodGet, "/callback", authSvc.GithubLoginCallbackHandler)
	authGithubRouter.Add(http.MethodGet, "/login", authSvc.LoginWithGithub)

	orgModeRouter := e.Group("/org", authSvc.JWTRest(), orgModeSvc.AllowOrgAdmin())

	RegisterUserRoutes(userApiRouter, userApi)
	RegisterNSRoutes(nsRouter, reg, registryStore, logger)
	RegisterAuthRoutes(authRouter, authSvc)
	Extensions(v2Router, reg, ext)
	RegisterWebauthnRoutes(webauthnRouter, webauthnServer)
	RegisterOrgModeRoutes(orgModeRouter, orgModeSvc)

	//catch-all will redirect user back to web interface
	e.Add(http.MethodGet, "/", func(ctx echo.Context) error {
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

		return ctx.Redirect(http.StatusTemporaryRedirect, webAppURL)
	})
}

// RegisterNSRoutes is one of the helper functions to Register
// it works directly with registry endpoints
func RegisterNSRoutes(
	nsRouter *echo.Group,
	reg registry.Registry,
	registryStore registry_store.RegistryStore,
	logger telemetry.Logger,
) {

	// ALL THE HEAD METHODS //
	// HEAD /v2/<name>/blobs/<digest>
	// (LayerExists) should be called reference/digest
	nsRouter.Add(http.MethodHead, BlobsDigest, reg.LayerExists)

	// HEAD /v2/<name>/manifests/<reference>
	// should be called reference/digest
	nsRouter.Add(http.MethodHead, ManifestsReference, reg.ManifestExists, registryReferenceOrTagValidator(logger))

	// ALL THE PUT METHODS

	// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	nsRouter.Add(http.MethodPut, BlobsUploadsUUID, reg.CompleteUpload)

	nsRouter.Add(http.MethodPut, BlobsMonolithicPut, reg.MonolithicPut)

	// PUT /v2/<name>/manifests/<reference>
	nsRouter.Add(
		http.MethodPut,
		ManifestsReference,
		reg.PushManifest,
		registryReferenceOrTagValidator(logger),
		propagateRepository(registryStore, logger),
	)

	// POST METHODS

	// POST /v2/<name>/blobs/uploads/
	nsRouter.Add(http.MethodPost, BlobsUploads, reg.StartUpload)

	// POST /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodPost, BlobsUploadsUUID, reg.PushLayer)

	// PATCH METHODS

	// PATCH /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodPatch, BlobsUploadsUUID, reg.ChunkedUpload)
	// router.Add(http.MethodPatch, "/blobs/uploads/", reg.ChunkedUpload)

	// GET METHODS

	// GET /v2/<name>/manifests/<reference>
	nsRouter.Add(http.MethodGet, ManifestsReference, reg.PullManifest, registryReferenceOrTagValidator(logger))

	// GET /v2/<name>/blobs/<digest>
	nsRouter.Add(http.MethodGet, BlobsDigest, reg.PullLayer)

	// GET /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodGet, BlobsUploadsUUID, reg.UploadProgress)

	// router.Add(http.MethodGet, "/blobs/:digest", reg.DownloadBlob)

	///GET /v2/<name>/tags/list
	nsRouter.Add(http.MethodGet, TagsList, reg.ListTags)

	///GET /v2/<name>/tags/list
	nsRouter.Add(http.MethodGet, GetReferrers, reg.ListReferrers)
	/// mf/sha -> mf/latest
	nsRouter.Add(http.MethodDelete, BlobsDigest, reg.DeleteLayer)
	nsRouter.Add(http.MethodDelete, ManifestsReference, reg.DeleteTagOrManifest, registryReferenceOrTagValidator(logger))
}

// Extensions for teh OCI dist spec
func Extensions(group *echo.Group, reg registry.Registry, ext extensions.Extenion, middlewares ...echo.MiddlewareFunc) {

	// GET /v2/_catalog
	group.Add(http.MethodGet, Catalog, reg.Catalog)
	group.Add(http.MethodGet, PublicCatalog, ext.PublicCatalog)
	// Auto-complete image search
	group.Add(http.MethodGet, Search, reg.GetImageNamespace)
	group.Add(http.MethodGet, CatalogDetail, ext.CatalogDetail, middlewares...)
	group.Add(http.MethodGet, RepositoryDetail, ext.RepositoryDetail, middlewares...)
	group.Add(http.MethodGet, UserCatalog, ext.GetUserCatalog, middlewares...)
	group.Add(http.MethodPost, ChangeRepositoryVisibility, ext.ChangeContainerImageVisibility, middlewares...)
	group.Add(http.MethodPost, CreateRepository, reg.CreateRepository, middlewares...)
}

func RegisterHealthCheckEndpoint(e *echo.Echo, fn http.HandlerFunc) {
	e.Add(http.MethodGet, "/health", echo.WrapHandler(fn))
}
