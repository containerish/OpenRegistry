package router

import (
	"github.com/containerish/OpenRegistry/store/postgres"
	"log"
	"net/http"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Register is the entry point that registers all the endpoints
// nolint
func Register(
	cfg *config.RegistryConfig, e *echo.Echo, reg registry.Registry, authSvc auth.Authentication,
	localCache cache.Store, pStore postgres.PersistentStore,
	) {
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		Generator: func() string {
			requestId := uuid.New()
			return requestId.String()
		},
	}))

	e.HideBanner = true

	p := prometheus.NewPrometheus("OpenRegistry", nil)
	p.Use(e)

	v2Router := e.Group(V2, authSvc.BasicAuth(), authSvc.JWT())
	nsRouter := v2Router.Group(Namespace, authSvc.ACL())

	internal := e.Group(Internal)
	authRouter := e.Group(Auth)
	betaRouter := e.Group(Beta)

	v2Router.Add(http.MethodGet, Root, reg.ApiVersion)
	e.Add(http.MethodGet, TokenAuth, authSvc.Token)

	RegisterNSRoutes(nsRouter, reg)
	RegisterAuthRoutes(authRouter, authSvc)
	RegisterBetaRoutes(betaRouter, localCache)
	InternalRoutes(internal, localCache)
	Docker(v2Router, reg)
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

// Docker is used for Catalog api
func Docker(group *echo.Group, reg registry.Registry) {

	// GET /v2/_catalog
	group.Add(http.MethodGet, Catalog, reg.Catalog)
}
