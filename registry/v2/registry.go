package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/types"
	"github.com/docker/distribution/uuid"
	"github.com/labstack/echo/v4"
)

func NewRegistry(
	skynetClient *skynet.Client,
	c cache.Store,
	logger telemetry.Logger,
	pgStore postgres.PersistentStore,
) (Registry, error) {
	r := &registry{
		debug:  true,
		skynet: skynetClient,
		b: blobs{
			mutex:    sync.Mutex{},
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
			layers:   map[string][]string{},
		},
		localCache: c,
		logger:     logger,
		mu:         &sync.RWMutex{},
		store:      pgStore,
		txnMap:     map[string]TxnStore{},
	}

	r.b.registry = r

	return r, nil
}

// Catalog - The list of available repositories is made available through the catalog.
//GET /v2/_catalog
func (r *registry) Catalog(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	bz, err := r.localCache.ListAll()
	if err != nil {
		logMsg := echo.Map{
			"error": err.Error(),
		}

		ctx.Set(types.HttpEndpointErrorKey, logMsg)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	var md []types.Metadata
	err = json.Unmarshal(bz, &md)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	}

	var result []string
	for _, el := range md {
		var repo []string
		ns := el.Namespace

		for _, c := range el.Manifest.Config {
			repo = append(repo, fmt.Sprintf("%s:%s", ns, c.Reference))
		}

		result = append(result, repo...)
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"repositories": result,
	})
}

func (r *registry) DeleteTagOrManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")

	if ref == "" {
		reqURI := strings.Split(ctx.Request().RequestURI, "/")
		if len(reqURI) == 6 {
			ref = reqURI[5]
		}
	}

	if err := r.localCache.UpdateManifestRef(namespace, ref); err != nil {
		details := map[string]interface{}{
			"namespace": namespace,
			"digest":    ref,
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), details)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	return ctx.NoContent(http.StatusAccepted)
}

func (r *registry) DeleteLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	dig := ctx.Param("digest")

	var m types.Metadata
	_, err := r.localCache.GetDigest(dig)
	if err != nil {
		bz, err := r.localCache.Get([]byte(namespace))
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)
			return ctx.JSONBlob(http.StatusNotFound, errMsg)
		}
		if err = json.Unmarshal(bz, &m); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)
			return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		}
	}

	err = r.localCache.DeleteLayer(namespace, dig)
	if err != nil {
		logMsg := echo.Map{
			"error":  err.Error(),
			"caller": "DeleteLayer",
		}

		bz, err := json.Marshal(logMsg)
		if err == nil {
			ctx.Set(types.HttpEndpointErrorKey, logMsg)
		}
		return ctx.JSONBlob(http.StatusInternalServerError, bz)
	}

	if err = r.localCache.DeleteDigest(dig); err != nil {
		logMsg := echo.Map{
			"error":  err.Error(),
			"caller": "DeleteLayer",
		}

		ctx.Set(types.HttpEndpointErrorKey, logMsg)
		bz, err := json.Marshal(logMsg)
		if err != nil {
			ctx.Set(types.HttpEndpointErrorKey, err.Error())
			r.logger.Log(ctx)
		}
		return ctx.JSONBlob(http.StatusInternalServerError, bz)
	}
	return ctx.NoContent(http.StatusAccepted)
}

// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) MonolithicUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")
	digest := ctx.QueryParam("digest")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	ctx.Request().Body.Close()

	link, err := r.skynet.Upload(namespace, digest, bz, true)
	if err != nil {
		detail := echo.Map{
			"error":  err.Error(),
			"caller": "MonolithicUpload",
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), detail)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusInternalServerError, bz)
	}

	metadata := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 2,
			MediaType:     "",
			Layers:        []*types.Layer{{MediaType: "", Size: len(bz), Digest: digest, SkynetLink: link, UUID: uuid}},
		},
	}

	err = r.localCache.Update([]byte(namespace), metadata.Bytes())
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	locationHeader := link
	ctx.Response().Header().Set("Location", locationHeader)
	return ctx.NoContent(http.StatusCreated)
}

// CompleteUpload
//PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
/*
for postgres:
this is where we insert into the layer after all the blobs have been accumulated
and inserted in the blob table
thus committing the txn
*/
func (r *registry) CompleteUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	dig := ctx.QueryParam("digest")
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	_ = ctx.Request().Body.Close()
	// insert if bz is not nil
	buf := bytes.NewBuffer(r.b.uploads[uuid])
	buf.Write(bz)
	ourHash := digest(buf.Bytes())
	delete(r.b.uploads, uuid)

	if ourHash != dig {
		details := map[string]interface{}{
			"headerDigest": dig, "serverSideDigest": ourHash, "bodyDigest": digest(bz),
		}
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, "digest mismatch", details)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	blobNamespace := fmt.Sprintf("%s/blobs", namespace)
	skylink, err := r.skynet.Upload(blobNamespace, dig, buf.Bytes(), true)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}
	if err := r.localCache.SetDigest(ourHash, skylink); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	}
	txnOp, ok := r.txnMap[uuid]
	layer := &types.LayerV2{
		MediaType:   "",
		Digest:      dig,
		SkynetLink:  skylink,
		UUID:        uuid,
		BlobDigests: txnOp.blobDigests,
		Size:        len(bz),
	}
	if !ok {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, "transaction does not exist for uuid -"+uuid, nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	if err := r.store.SetLayer(ctx.Request().Context(), txnOp.txn, layer); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	// val := &types.ImageManifestV2{
	// 	Uuid:          uuid,
	// 	Namespace:     namespace,
	// 	MediaType:     "",
	// 	SchemaVersion: 2,
	// }

	//if err = r.localCache.Update([]byte(namespace), val.Bytes()); err != nil {
	//	errMsg := r.errorResponse(RegistryErrorCodeUnsupported, err.Error(), nil)
	//	ctx.Set(types.HttpEndpointErrorKey, errMsg)
	//	return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	//}

	// if err := r.store.SetManifest(ctx.Request().Context(), txnOp.txn, val); err != nil {
	// 	errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
	// 	ctx.Set(types.HttpEndpointErrorKey, errMsg)
	// 	return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	// }

	if err := r.store.Commit(ctx.Request().Context(), txnOp.txn); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	delete(r.txnMap, uuid)

	locationHeader := fmt.Sprintf("/v2/%s/blobs/%s", namespace, ourHash)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Content-Digest", ourHash)
	ctx.Response().Header().Set("Location", locationHeader)
	return ctx.NoContent(http.StatusCreated)
}

// HEAD /v2/<name>/blobs/<digest>
// 200 OK
// Content-Length: <length of blob>
// Docker-Content-Digest: <digest>
func (r *registry) LayerExists(ctx echo.Context) error {
	return r.b.HEAD(ctx)
}

// HEAD /v2/<name>/manifests/<reference>
func (r *registry) ManifestExists(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference") // ref can be either tag or digest

	skylink, err := r.localCache.ResolveManifestRef(namespace, ref)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
			"error":  err.Error(),
		}

		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	size, ok := r.skynet.Metadata(skylink)
	if !ok {
		lm := logMsg{
			"error":   "metadata not found for skylink",
			"skylink": skylink,
		}
		r.logger.Error(lm)
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	bz, err := r.localCache.Get([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	var md types.Metadata
	if err = json.Unmarshal(bz, &md); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		lm := logMsg{
			"errorUnmarshal": fmt.Sprintf("%s\n", errMsg),
		}
		r.logger.Error(lm)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	manifest, err := md.GetManifestByRef(ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	if manifest.Reference != ref && manifest.Digest != ref {
		details := map[string]interface{}{
			"foundDigest":  manifest.Digest,
			"clientDigest": ref,
		}
		r.logger.Error(details)
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, "manifest digest does not match", nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	ctx.Response().Header().Set("Content-Type", "application/json")
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)

	return ctx.NoContent(http.StatusOK)
}

// ChunkedUpload
// PATCH /v2/<name>/blobs/uploads/<uuid>
func (r *registry) ChunkedUpload(ctx echo.Context) error {
	return r.b.UploadBlob(ctx)
}

func (r *registry) CancelUpload(ctx echo.Context) error {
	return nil
}

// PullManifest GET /v2/<name>/manifests/<reference>
func (r *registry) PullManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")

	bz, err := r.localCache.Get([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		err = ctx.JSONBlob(http.StatusNotFound, errMsg)
		return err
	}

	var md types.Metadata
	if err = json.Unmarshal(bz, &md); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	skynetLink, err := r.localCache.ResolveManifestRef(namespace, ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	resp, err := r.skynet.Download(skynetLink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	bz, err = io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	_ = resp.Close()

	manifest, err := md.GetManifestByRef(ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	ctx.Response().Header().Set("X-Docker-Content-ID", skynetLink)
	ctx.Response().Header().Set("Content-Type", manifest.MediaType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", manifest.Size))
	return ctx.JSONBlob(http.StatusOK, bz)
}

func (r *registry) PushManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")
	contentType := ctx.Request().Header.Get("Content-Type")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	ctx.Request().Body.Close()

	dig := digest(bz)

	var manifest ManifestList
	if err = json.Unmarshal(bz, &manifest); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	color.Red("manifest list: %s\n", bz)

	mfNamespace := fmt.Sprintf("%s/manifests", namespace)
	skylink, err := r.skynet.Upload(mfNamespace, dig, bz, true)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	//id := uuid.Generate()
	//mfc := &types.ConfigV2{
	//	UUID:      id.String(),
	//	Namespace: namespace,
	//	Reference: ref,
	//	Digest:    dig,
	//	Skylink:   skylink,
	//	MediaType: contentType,
	//	Layers:    nil,
	//	Size:      0,
	//}

	manifestConfig := &types.Config{
		MediaType:  contentType,
		Size:       len(bz),
		Digest:     dig,
		SkynetLink: skylink,
		Reference:  ref,
	}

	metadata := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 2,
			Config:        []*types.Config{manifestConfig},
		},
	}

	if err = r.localCache.Update([]byte(namespace), metadata.Bytes()); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	val := &types.ImageManifestV2{
		Uuid:          uuid.Generate().String(),
		Namespace:     namespace,
		MediaType:     "",
		SchemaVersion: 2,
	}

	//if err = r.localCache.Update([]byte(namespace), val.Bytes()); err != nil {
	//	errMsg := r.errorResponse(RegistryErrorCodeUnsupported, err.Error(), nil)
	//	ctx.Set(types.HttpEndpointErrorKey, errMsg)
	//	return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	//}
	txnOp, _ := r.store.NewTxn(context.Background())

	if err := r.store.SetManifest(ctx.Request().Context(), txnOp, val); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	locationHeader := r.getHttpUrlFromSkylink(skylink)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	ctx.Response().Header().Set("X-Docker-Content-ID", skylink)

	return ctx.String(http.StatusCreated, "Created")
}

// Content discovery GET /v2/<name>/tags/list

func (r *registry) ListTags(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	limit := ctx.QueryParam("n")

	l, err := r.localCache.ListWithPrefix([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	var md types.Metadata
	err = json.Unmarshal(l, &md)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	var tags []string
	for _, v := range md.Manifest.Config {
		tags = append(tags, v.Reference)
	}
	if limit != "" {
		n, err := strconv.ParseInt(limit, 10, 32)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusNotFound, errMsg)
		}
		if n > 0 {
			tags = tags[0:n]
		}
		if n == 0 {
			tags = []string{}
		}
	}

	sort.Strings(tags)
	return ctx.JSON(http.StatusOK, echo.Map{
		"name": namespace,
		"tags": tags,
	})
}
func (r *registry) List(ctx echo.Context) error {
	return fmt.Errorf("error")
}

// GET /v2/<name>/blobs/<digest>

func (r *registry) PullLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.Param("digest")

	layerRef, err := r.localCache.GetDigest(clientDigest)
	if err != nil {
		skynetLink, err := r.localCache.GetSkynetURL(namespace, clientDigest)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusNotFound, errMsg)
		}
		layerRef = &types.LayerRef{
			Digest:  clientDigest,
			Skylink: skynetLink,
		}
	}

	size, ok := r.skynet.Metadata(layerRef.Skylink)
	if ok {
		url := fmt.Sprintf("https://siasky.net/%s",
			strings.Replace(layerRef.Skylink, "sia://", "", 1))
		ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
		http.Redirect(ctx.Response(), ctx.Request(), url, http.StatusTemporaryRedirect)
		return nil
	}

	detail := map[string]interface{}{
		"error": "skylink is empty",
	}
	e := fmt.Errorf("skylink is empty").Error()
	errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, e, detail)
	ctx.Set(types.HttpEndpointErrorKey, errMsg)

	return ctx.JSONBlob(http.StatusNotFound, errMsg)
}

//BlobMount to be implemented by guacamole at a later stage
func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}

//PushImage is already implemented through StartUpload and ChunkedUpload
func (r *registry) PushImage(ctx echo.Context) error {
	return nil
}

/*StartUpload
for postgres:
start a tnx
registry.tnxMap[uuid] = {txn,blobs[],timeout}
*/
func (r *registry) StartUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.QueryParam("digest")

	if clientDigest != "" {
		bz, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			details := map[string]interface{}{
				"clientDigest": clientDigest,
				"namespace":    namespace,
			}
			errMsg := r.errorResponse(
				RegistryErrorCodeBlobUploadInvalid,
				"error while reading request body",
				details,
			)

			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusNotFound, errMsg)

		}
		ctx.Request().Body.Close() // why defer? body is already read :)
		dig := digest(bz)

		if dig != clientDigest {
			details := map[string]interface{}{
				"clientDigest":   clientDigest,
				"computedDigest": dig,
			}
			errMsg := r.errorResponse(
				RegistryErrorCodeDigestInvalid,
				"client digest does not meet computed digest",
				details,
			)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusBadRequest, errMsg)
		}

		skylink, err := r.skynet.Upload(namespace, dig, bz, true)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		}

		layer := &types.Layer{
			MediaType:  "application/vnd.docker.image.rootfs.diff.tar.gzip",
			Size:       len(bz),
			Digest:     dig,
			SkynetLink: skylink,
			UUID:       "",
		}

		var val types.Metadata
		val.Namespace = namespace
		val.Manifest.Layers = append(val.Manifest.Layers, layer)

		link := r.getHttpUrlFromSkylink(skylink)
		if err = r.localCache.Update([]byte(namespace), val.Bytes()); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeUnsupported, err.Error(), nil)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)

			return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		}
		ctx.Response().Header().Set("Location", link)

		return ctx.NoContent(http.StatusCreated)
	}

	id := uuid.Generate()
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, id.String())
	txn, err := r.store.NewTxn(ctx.Request().Context())
	if err != nil {
		errMsg := r.errorResponse(
			RegistryErrorCodeUnknown,
			err.Error(),
			nil,
		)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	}
	r.txnMap[id.String()] = TxnStore{
		txn:         txn,
		blobDigests: []string{},
		timeout:     time.Minute * 30,
	}
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Upload-UUID", id.String())
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", 0))

	return ctx.NoContent(http.StatusAccepted)
}

func (r *registry) UploadProgress(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")

	skylink, err := r.localCache.GetSkynetURL(namespace, uuid)
	if err != nil {
		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", "bytes=0-0")
		ctx.Response().Header().Set("Docker-Upload-UUID", uuid)

		return ctx.NoContent(http.StatusNoContent)
	}

	size, ok := r.skynet.Metadata(skylink)
	if !ok {
		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", "bytes=0-0")
		ctx.Response().Header().Set("Docker-Upload-UUID", uuid)

		return ctx.NoContent(http.StatusNoContent)
	}

	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("bytes=0-%d", size))
	ctx.Response().Header().Set("Docker-Upload-UUID", uuid)

	return ctx.NoContent(http.StatusNoContent)
}

// POST /v2/<name>/blobs/uploads/
func (r *registry) PushLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	elem := strings.Split(ctx.Request().URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		errMsg := r.errorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)

		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	id := uuid.Generate()
	p := path.Join(elem[1 : len(elem)-2]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, id.String())
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", id.String())
	ctx.Response().Header().Set("Range", "bytes=0-0")

	return ctx.NoContent(http.StatusAccepted)
}

// Should also look into 401 Code
// https://docs.docker.com/registry/spec/api/
func (r *registry) ApiVersion(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		r.logger.Log(ctx).Send()
	}()

	ctx.Response().Header().Set(HeaderDockerDistributionApiVersion, "registry/2.0")

	return ctx.String(http.StatusOK, "OK\n")
}
