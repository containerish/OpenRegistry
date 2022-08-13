package router

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	"github.com/fatih/color"
	"github.com/google/go-github/v42/github"
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
	ext extensions.Extenion,
) {
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     strings.Split(cfg.WebAppEndpoint, ","),
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
	}))

	e.HideBanner = true

	p := prometheus.NewPrometheus("OpenRegistry", nil)
	p.Use(e)

	e.Add(http.MethodPost ,"/github/connect/setup", func(ctx echo.Context) error {
		bz, _ := io.ReadAll(ctx.Request().Body)
		color.Red("output: %s",bz)
		prEvent := github.PullRequestEvent{}
		err := json.NewDecoder(ctx.Request().Body).Decode(&prEvent)
		if err != nil {
			
		}
		prEvent.GetPullRequest()

		return nil
})
	v2Router := e.Group(V2, authSvc.BasicAuth(), authSvc.JWT())
	nsRouter := v2Router.Group(Namespace, authSvc.ACL())

	authRouter := e.Group(Auth)
	githubRouter := authRouter.Group("/github")

	v2Router.Add(http.MethodGet, Root, reg.ApiVersion)

	e.Add(http.MethodGet, TokenAuth, authSvc.Token)

	githubRouter.Add(http.MethodGet, "/callback", authSvc.GithubLoginCallbackHandler)
	githubRouter.Add(http.MethodGet, "/login", authSvc.LoginWithGithub)

	RegisterNSRoutes(nsRouter, reg)
	RegisterAuthRoutes(authRouter, authSvc)
	Extensions(v2Router, reg, ext, authSvc.JWT())

	//catch-all will redirect user back to web interface
	e.Add(http.MethodGet, "/", func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusTemporaryRedirect, cfg.WebAppEndpoint)
	})
}

// RegisterNSRoutes is one of the helper functions to Register
// it works directly with registry endpoints
func RegisterNSRoutes(nsRouter *echo.Group, reg registry.Registry) {

	// ALL THE HEAD METHODS //
	// HEAD /v2/<name>/blobs/<digest>
	// (LayerExists) should be called reference/digest
	nsRouter.Add(http.MethodHead, BlobsDigest, reg.LayerExists)

	// HEAD /v2/<name>/manifests/<reference>
	// should be called reference/digest
	nsRouter.Add(http.MethodHead, ManifestsReference, reg.ManifestExists)

	// ALL THE PUT METHODS
	// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	nsRouter.Add(http.MethodPut, BlobsUploadsUUID, reg.CompleteUpload)

	// PUT /v2/<name>/manifests/<reference>
	nsRouter.Add(http.MethodPut, ManifestsReference, reg.PushManifest)

	// POST METHODS
	// POST /v2/<name>/blobs/uploads/
	nsRouter.Add(http.MethodPost, BlobsUploads, reg.StartUpload)

	// POST /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodPost, BlobsUploadsUUID, reg.PushLayer)

	// PATCH

	// PATCH /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodPatch, BlobsUploadsUUID, reg.ChunkedUpload)
	// router.Add(http.MethodPatch, "/blobs/uploads/", reg.ChunkedUpload)

	// GET
	// GET /v2/<name>/manifests/<reference>
	nsRouter.Add(http.MethodGet, ManifestsReference, reg.PullManifest)

	// GET /v2/<name>/blobs/<digest>
	nsRouter.Add(http.MethodGet, BlobsDigest, reg.PullLayer)

	// GET GET /v2/<name>/blobs/uploads/<uuid>
	nsRouter.Add(http.MethodGet, BlobsUploadsUUID, reg.UploadProgress)

	// router.Add(http.MethodGet, "/blobs/:digest", reg.DownloadBlob)

	///GET /v2/<name>/tags/list
	nsRouter.Add(http.MethodGet, TagsList, reg.ListTags)

	/// mf/sha -> mf/latest
	nsRouter.Add(http.MethodDelete, BlobsDigest, reg.DeleteLayer)
	nsRouter.Add(http.MethodDelete, ManifestsReference, reg.DeleteTagOrManifest)
}

// Extensions for teh OCI dist spec
func Extensions(group *echo.Group, reg registry.Registry, ext extensions.Extenion, middlewares ...echo.MiddlewareFunc) {

	// GET /v2/_catalog
	group.Add(http.MethodGet, Catalog, reg.Catalog)
	// Auto-complete image search
	group.Add(http.MethodGet, Search, reg.GetImageNamespace)
	group.Add(http.MethodGet, CatalogDetail, ext.CatalogDetail, middlewares...)
	group.Add(http.MethodGet, RepositoryDetail, ext.RepositoryDetail, middlewares...)
}
