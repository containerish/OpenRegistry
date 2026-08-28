package router

import (
	"net/http"

	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/registry/v2/extensions"
	registry_store "github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

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
	nsRouter.Add(
		http.MethodDelete,
		ManifestsReference,
		reg.DeleteTagOrManifest,
		registryReferenceOrTagValidator(logger),
	)
}

// RegisterExtensionsRoutes for teh OCI dist spec
func RegisterExtensionsRoutes(
	group *echo.Group,
	reg registry.Registry,
	ext extensions.Extenion,
	middlewares ...echo.MiddlewareFunc,
) {

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
	group.Add(http.MethodPost, RepositoryFavorites, ext.AddRepositoryToFavorites, middlewares...)
	group.Add(http.MethodGet, RepositoryFavorites, ext.ListFavoriteRepositories, middlewares...)
	group.Add(http.MethodDelete, RemoveRepositoryFavorites, ext.RemoveRepositoryFromFavorites, middlewares...)
}
