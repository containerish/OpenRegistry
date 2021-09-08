package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	skynetsdk "github.com/NebulousLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/types"
	"github.com/docker/distribution/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func NewRegistry(skynetClient *skynet.Client, logger zerolog.Logger, c cache.Store, echoLogger echo.Logger) (Registry, error) {
	r := &registry{
		log:    logger,
		debug:  true,
		skynet: skynetClient,
		b: blobs{
			mutex:    sync.Mutex{},
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
			layers:   map[string][]string{},
		},
		localCache: c,
		echoLogger: echoLogger,
		mu:         &sync.RWMutex{},
	}

	r.b.registry = r

	return r, nil
}

//DeleteLayer is still under progress --guacamole will work on it a later stage
func (r *registry) DeleteLayer(ctx echo.Context) error {
	dig := ctx.Param("digest")

	_, err := r.localCache.GetDigest(dig)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	return ctx.NoContent(http.StatusAccepted)
}

// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) MonolithicUpload(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")
	digest := ctx.QueryParam("digest")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	ctx.Request().Body.Close()

	link, err := r.skynet.Upload(namespace, digest, bz)
	if err != nil {
		return err
	}

	metadata := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 2,
			MediaType:     "",
			Layers:        []*types.Layer{{MediaType: "", Size: len(bz), Digest: digest, SkynetLink: link, UUID: uuid}},
		},
	}

	r.localCache.Update([]byte(namespace), metadata.Bytes())
	locationHeader := link
	ctx.Response().Header().Set("Location", locationHeader)
	return ctx.NoContent(http.StatusCreated)
}

// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) CompleteUpload(ctx echo.Context) error {
	dig := ctx.QueryParam("digest")
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	ctx.Request().Body.Close()

	buf := bytes.NewBuffer(r.b.uploads[uuid])
	buf.Write(bz)
	ourHash := digest(buf.Bytes())
	delete(r.b.uploads, uuid)

	if ourHash != dig {
		details := map[string]interface{}{
			"headerDigest": dig, "serverSideDigest": ourHash, "bodyDigest": digest(bz),
		}
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, "digest mismatch", details)
		r.debugf(details)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	var headers []skynetsdk.Header
	for k, v := range ctx.Request().Header {
		headers = append(headers, skynetsdk.Header{
			Key: k, Value: v[0],
		})
	}

	blobNamespace := fmt.Sprintf("%s/blobs", namespace)
	skylink, err := r.skynet.Upload(blobNamespace, dig, buf.Bytes(), headers...)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}
	if err := r.localCache.SetDigest(ourHash, skylink); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	}

	val := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 2,
			MediaType:     "",
			Layers: []*types.Layer{
				{
					MediaType: "", Size: len(bz), Digest: dig, SkynetLink: skylink, UUID: uuid,
				},
			},
		},
	}

	r.localCache.Update([]byte(namespace), val.Bytes())

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
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference") // ref can be either tag or digest

	skylink, err := r.localCache.ResolveManifestRef(namespace, ref)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
			"error":  err.Error(),
		}

		r.debugf(logMsg(details))
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	size, ok := r.skynet.Metadata(skylink)
	if !ok {
		lm := logMsg{
			"error":   "metadata not found for skylink",
			"skylink": skylink,
		}
		r.debugf(lm)
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	bz, err := r.localCache.Get([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		lm := logMsg{
			"errorGetCache": fmt.Sprintf("%s\n", errMsg),
		}
		r.debugf(lm)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	var md types.Metadata
	if err = json.Unmarshal(bz, &md); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		lm := logMsg{
			"errorUnmarshal": fmt.Sprintf("%s\n", errMsg),
		}
		r.debugf(lm)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	manifest, err := md.GetManifestByRef(ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	if manifest.Reference != ref && manifest.Digest != ref {
		details := map[string]interface{}{
			"foundDigest":  manifest.Digest,
			"clientDigest": ref,
		}
		r.debugf(details)
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, "manifest digest does not match", nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	ctx.Response().Header().Set("Content-Type", "application/json")
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	return ctx.NoContent(http.StatusOK)
}

// PATCH /v2/<name>/blobs/uploads/<uuid>
func (r *registry) ChunkedUpload(ctx echo.Context) error {
	return r.b.UploadBlob(ctx)
}

func (r *registry) CancelUpload(ctx echo.Context) error {
	return nil
}

// PullManifest GET /v2/<name>/manifests/<reference>
func (r *registry) PullManifest(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")

	bz, err := r.localCache.Get([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	var md types.Metadata
	if err = json.Unmarshal(bz, &md); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	skynetLink, err := r.localCache.ResolveManifestRef(namespace, ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	resp, err := r.skynet.Download(skynetLink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	bz, err = io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}
	_ = resp.Close()

	manifest, err := md.GetManifestByRef(ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	ctx.Response().Header().Set("X-Docker-Content-ID", skynetLink)
	ctx.Response().Header().Set("Content-Type", manifest.MediaType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", manifest.Size))
	return ctx.JSONBlob(http.StatusOK, bz)
}

func (r *registry) PushManifest(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")
	contentType := ctx.Request().Header.Get("Content-Type")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}
	ctx.Request().Body.Close()

	dig := digest(bz)

	var manifest ManifestList
	if err = json.Unmarshal(bz, &manifest); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	mfNamespace := fmt.Sprintf("%s/manifests", namespace)
	skylink, err := r.skynet.Upload(mfNamespace, dig, bz)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

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
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	locationHeader := r.getHttpUrlFromSkylink(skylink)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	ctx.Response().Header().Set("X-Docker-Content-ID", skylink)
	return ctx.String(http.StatusCreated, "Created")
}

func (r *registry) DeleteImage(ctx echo.Context) error {
	clientDigest := ctx.Param("digest")
	if clientDigest == "" {
		reqURI := strings.Split(ctx.Request().RequestURI, "/")
		if len(reqURI) == 6 {
			clientDigest = reqURI[5]
		}
	}
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")

	if d, err := r.localCache.ResolveManifestRef(namespace, clientDigest); err != nil {
		details := map[string]interface{}{
			"namespace": namespace,
			"digest":    clientDigest,
			"data":      d,
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), details)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	return ctx.NoContent(http.StatusAccepted)
}

// GET /v2/<name>/blobs/<digest>
func (r *registry) PullLayer(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.Param("digest")

	skynetLink, err := r.localCache.GetSkynetURL(namespace, clientDigest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	resp, err := r.skynet.Download(skynetLink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	bz, err := io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	}
	_ = resp.Close()

	dig := digest(bz)
	if dig != clientDigest {
		details := map[string]interface{}{
			"clientDigest":   clientDigest,
			"computedDigest": dig,
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadUnknown, "client digest is different than computed digest", details)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(bz)))
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	return ctx.Blob(http.StatusOK, "application/octet-stream", bz)
}

//BlobMount to be implemented by guacamole at a later stage
func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}

//PushImage is already implemented through StartUpload and ChunkedUpload
func (r *registry) PushImage(ctx echo.Context) error {
	return nil
}

func (r *registry) StartUpload(ctx echo.Context) error {

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.QueryParam("digest")

	if clientDigest != "" {
		var headers []skynetsdk.Header

		for k, v := range ctx.Request().Header {
			headers = append(headers, skynetsdk.Header{
				Key: k, Value: v[0],
			})
		}

		bz, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			panic(err)
		}
		ctx.Request().Body.Close() // why defer? body is already read :)
		dig := digest(bz)

		if dig != clientDigest {
			details := map[string]interface{}{
				"clientDigest":   clientDigest,
				"computedDigest": dig,
			}
			errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, "client digest does not meet computed digest", details)
			return ctx.JSONBlob(http.StatusBadRequest, errMsg)
		}

		skylink, err := r.skynet.Upload(namespace, dig, bz, headers...)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
			lm := logMsg{
				"error":  err.Error(),
				"digest": dig,
			}
			r.debugf(lm)
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
		r.localCache.Update([]byte(namespace), val.Bytes())
		ctx.Response().Header().Set("Location", link)
		return ctx.NoContent(http.StatusAccepted)
	}

	id := uuid.Generate()
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, id.String())

	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Upload-UUID", id.String())
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", 0))
	return ctx.NoContent(http.StatusAccepted)
}

func (r *registry) UploadProgress(ctx echo.Context) error {
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

	elem := strings.Split(ctx.Request().URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		errMsg := r.errorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
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

func (r *registry) ListTags(ctx echo.Context) error {
	return nil
}
func (r *registry) List(ctx echo.Context) error {
	return nil
}

// Should also look into 401 Code
// https://docs.docker.com/registry/spec/api/
func (r *registry) ApiVersion(ctx echo.Context) error {
	ctx.Response().Header().Set(HeaderDockerDistributionApiVersion, "registry/2.0")
	return ctx.String(http.StatusOK, "OK\n")
}
