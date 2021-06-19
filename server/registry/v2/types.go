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
	"github.com/docker/distribution/uuid"
	"github.com/jay-dee7/parachute/cache"
	"github.com/jay-dee7/parachute/skynet"
	"github.com/jay-dee7/parachute/types"
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

func (r *registry) DeleteLayer(ctx echo.Context) error {
	return nil
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
	defer ctx.Request().Body.Close()

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
			Config:        types.Config{},
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
	// contentRange := ctx.Request().Header.Get("Content-Range")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	buf := bytes.NewBuffer(r.b.uploads[uuid])
	buf.Write(bz)
	// io.Copy(buf, ctx.Request().Body)
	// io.CopyN(buf, ctx.Request().Body, ctx.Request().ContentLength-1)

	ourHash := digest(buf.Bytes())

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

	r.b.contents[ourHash] = buf.Bytes()

	blobNamespace := fmt.Sprintf("%s/blobs", namespace)
	skylink, err := r.skynet.Upload(blobNamespace, dig, buf.Bytes(), headers...)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	// delete(r.b.uploads, ref)

	val := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 2,
			MediaType:     "",
			Layers:        []*types.Layer{{MediaType: "", Size: len(bz), Digest: dig, SkynetLink: skylink, UUID: uuid}},
			Config:        types.Config{},
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

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	digest := ctx.Param("digest") // ref can be either tag or digest

	skylink, err := r.localCache.GetSkynetURL(namespace, digest)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	size, ok := r.skynet.Metadata(skylink)
	if !ok {
		lm := logMsg{
			"warn":    "metadata not found for skylink",
			"skylink": skylink,
		}
		r.debugf(lm)
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	bz, err := r.localCache.Get([]byte(namespace))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	var md types.Metadata
	if err = json.Unmarshal(bz, &md); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	return ctx.String(http.StatusOK, "OK")
}

// HEAD /v2/<name>/manifests/<reference>
func (r *registry) ManifestExists(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference") // ref can be either tag or digest

	skylink, err := r.localCache.ResolveManifestRef(namespace, ref)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
			"error": err.Error(),
		}

		r.debugf(logMsg(details))
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	size, ok := r.skynet.Metadata(skylink)
	if !ok {
		lm := logMsg{
			"warn":    "metadata not found for skylink",
			"skylink": skylink,
		}
		r.debugf(lm)
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
		// bz, err = r.localCache.Get([]byte(namespace))
		// if err != nil {
		// 	errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		// 	return ctx.JSON(http.StatusNotFound, errMsg)
		// }
		// var meta cache.Metadata
		// if err = json.Unmarshal(bz, &meta); err != nil {
		// 	details := echo.Map{
		// 		"error": err.Error(),
		// 	}
		// 	errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		// 	return ctx.JSON(http.StatusNotFound, errMsg)
		// }
		// meta.Find()
		// size = meta.Size
	}
	// }

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

	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		lm := logMsg{
			"error": fmt.Sprintf("%s\n", errMsg),
		}
		r.debugf(lm)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	if md.Manifest.Config.Reference != ref {
		details := map[string]interface{}{
			"foundDigest":  md.Manifest.Config.Digest,
			"clientDigest": ref,
		}
		r.debugf(details)
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, "manifest digest does not match", details)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", md.Manifest.Config.Digest)
	return ctx.String(http.StatusOK, "OK")
}

// PATCH /v2/<name>/blobs/uploads/<uuid>
func (r *registry) ChunkedUpload(ctx echo.Context) error {
	return r.b.UploadBlob(ctx)

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	chunkID := ctx.Param("uuid")

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
	defer ctx.Request().Body.Close()

	dig := digest(bz)
	skylink, err := r.skynet.Upload(namespace, dig, bz, headers...)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		lm := logMsg{
			"error":  err.Error(),
			"digest": dig,
			"uuid":   chunkID,
		}
		r.debugf(lm)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	val := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 0,
			MediaType:     "",
			Layers:        []*types.Layer{{SkynetLink: skylink, Size: len(bz), UUID: chunkID, Digest: dig}},
		},
	}

	r.localCache.Update([]byte(namespace), val.Bytes())

	// id := uuid.Generate()

	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, chunkID)

	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(bz)-1))
	ctx.Response().Header().Set("Docker-Upload-UUID", chunkID)
	return ctx.NoContent(http.StatusAccepted)
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
		ctx.JSON(http.StatusNotFound, errMsg)
	}

	resp, err := r.skynet.Download(skynetLink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}
	defer resp.Close()

	bz, err = io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Docker-Content-Digest", md.Manifest.Config.Digest)
	ctx.Response().Header().Set("X-Docker-Content-ID", skynetLink)
	ctx.Response().Header().Set("Content-Type", md.Manifest.Config.MediaType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", md.Manifest.Config.Size))
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
	defer ctx.Request().Body.Close()

	digest := digest(bz)

	var manifest ManifestList
	if err = json.Unmarshal(bz, &manifest); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	// skynetLink, err := r.skynet.Upload(namespace, ref, bz)
	// if err != nil {
	// 	errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
	// 	return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	// }

	// layers, ok := r.b.get(namespace)
	// if !ok {
	// 	errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "manifest ref not found", nil)
	// 	return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// }
	// r.b.remove(namespace)

	// refs := make(map[string][]byte)
	// refs[digest] = bz
	// refs["latest"] = bz
	// refs[ref] = bz

	mfNamespace := fmt.Sprintf("%s/manifests", namespace)
	skylink, err := r.skynet.Upload(mfNamespace, digest, bz)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	// skylink, err := r.skynet.AddImage(namespace, refs, layers)
	// if err != nil {
	// 	errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
	// 	return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// }

	metadata := types.Metadata{
		Namespace: namespace,
		Manifest: types.ImageManifest{
			SchemaVersion: 0,
			Config:        types.Config{MediaType: contentType, Size: len(bz), Digest: digest, SkynetLink: skylink, Reference: ref},
		},
	}


	r.localCache.Update([]byte(namespace), metadata.Bytes())
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	ctx.Response().Header().Set("X-Docker-Content-ID", skylink)
	return ctx.NoContent(http.StatusCreated)

	// digest := r.digest(bz)
	// mf := ImageManifest{
	// 	SchemaVersion: 2,
	// 	MediaType:     contentType,
	// 	Config:        Config{MediaType: "", Size: 0, Digest: digest},
	// }

	// bz, err := r.localCache.Get([]byte(namespace))
	// if err != nil {
	// 	errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
	// 	ctx.JSONBlob(http.StatusBadRequest, errMsg)
	// }

	// mf.Layers = append(mf.Layers, layer)

	// if mf.MediaType == string(types.OCIImageIndex) || mf.MediaType == string(types.DockerManifestList) {
	// 	var mfList ManifestList
	// 	if err := json.NewDecoder(ctx.Request().Body).Decode(&mfList); err != nil {
	// 		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
	// 		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// 	}

	// 	for _, desc := range mfList.Manifests {
	// 		val, _ := r.localCache.Get([]byte(namespace))
	// 		var m map[string]string
	// 		_ = json.Unmarshal(val, &m)
	// 		if _, found := m[desc.Digest]; !found {
	// 			errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, "sub-manifest not found: "+desc.Digest, nil)
	// 			return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// 		}
	// 	}
	// }

	// r.skynet.AddImage(manifests, layers)
}

func (r *registry) DeleteImage(ctx echo.Context) error {
	return nil
}

// GET /v2/<name>/blobs/<digest>
func (r *registry) PullLayer(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.Param("digest")

	skynetlink, err := r.localCache.GetSkynetURL(namespace, clientDigest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	// size, _ := r.skynet.Metadata(skynetlink)

	resp, err := r.skynet.Download(skynetlink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	defer resp.Close()

	bz, err := io.ReadAll(resp)
	_ = err

	// if resp.StatusCode != http.StatusOK {
	// 	errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
	// 	return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// }

	dig := digest(bz)
	if dig != clientDigest {
		details := map[string]interface{}{
			"clientDigest": clientDigest,
			"computedDigest": dig,
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadUnknown, "client digest is different than computed digest", details)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(bz)))
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	return ctx.Blob(http.StatusOK, "application/octet-stream", bz)
}

func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}

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

