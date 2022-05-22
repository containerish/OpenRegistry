package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/skynet"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func NewRegistry(
	skynetClient *skynet.Client,
	logger telemetry.Logger,
	pgStore postgres.PersistentStore,
) (Registry, error) {
	r := &registry{
		debug:  true,
		skynet: skynetClient,
		b: blobs{
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
			layers:   map[string][]string{},
		},
		logger: logger,
		store:  pgStore,
		txnMap: map[string]TxnStore{},
	}

	r.b.registry = r

	return r, nil
}

// LayerExists
// HEAD /v2/<name>/blobs/<digest>
// 200 OK
// Content-Length: <length of blob>
// Docker-Content-Digest: <digest>
// OK
func (r *registry) LayerExists(ctx echo.Context) error {
	return r.b.HEAD(ctx)
}

// ManifestExists
// HEAD /v2/<name>/manifests/<reference>
// OK
func (r *registry) ManifestExists(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference") // ref can be either tag or digest

	manifest, err := r.store.GetManifestByReference(ctx.Request().Context(), namespace, ref)

	if err != nil {
		details := echo.Map{
			"error":   err.Error(),
			"message": "skynet - manifest not found",
		}

		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	metadata, err := r.skynet.Metadata(manifest.Skylink)
	if err != nil {
		detail := map[string]interface{}{
			"error":   err.Error(),
			"skylink": manifest.Skylink,
		}

		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if manifest.Reference != ref && manifest.Digest != ref {
		details := map[string]interface{}{
			"foundDigest":  manifest.Digest,
			"clientDigest": ref,
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, "manifest digest does not match", details)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	ctx.Response().Header().Set("Content-Type", "application/json")
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", metadata.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	echoErr := ctx.NoContent(http.StatusOK)
	r.logger.Log(ctx, nil)
	return echoErr
}

// Catalog - The list of available repositories is made available through the catalog.
// GET /v2/_catalog?n=10&last=10&ns=johndoe
// OK
func (r *registry) Catalog(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	namespace := ctx.QueryParam("ns")
	var pageSize int64
	var offset int64
	if queryParamPageSize != "" {
		ps, err := strconv.ParseInt(ctx.QueryParam("n"), 10, 64)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			r.logger.Log(ctx, err)
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.ParseInt(ctx.QueryParam("last"), 10, 64)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			r.logger.Log(ctx, err)
			return echoErr
		}
		offset = o
	}

	catalogList, err := r.store.GetCatalog(ctx.Request().Context(), namespace, pageSize, offset)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	// empty namespace to pull the full catalog list
	total, err := r.store.GetCatalogCount(ctx.Request().Context(), "")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": catalogList,
		"total":        total,
	})
	r.logger.Log(ctx, nil)
	return echoErr

}

// ListTags Content discovery
// GET /v2/<name>/tags/list
// OK
func (r *registry) ListTags(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	limit := ctx.QueryParam("n")

	tags, err := r.store.GetImageTags(ctx.Request().Context(), namespace)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if limit != "" {
		n, err := strconv.ParseInt(limit, 10, 32)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}
		if n > 0 {
			tags = tags[0:n]
		}
		if n == 0 {
			tags = nil
		}
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"name": namespace,
		"tags": tags,
	})
	r.logger.Log(ctx, nil)
	return echoErr
}
func (r *registry) List(ctx echo.Context) error {
	return fmt.Errorf("error")
}

// PullManifest
// GET /v2/<name>/manifests/<reference>
// OK
func (r *registry) PullManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")

	manifest, err := r.store.GetManifestByReference(ctx.Request().Context(), namespace, ref)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	resp, err := r.skynet.Download(manifest.Skylink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	bz, err := io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = resp.Close()
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	ctx.Response().Header().Set("X-Docker-Content-ID", manifest.Skylink)
	ctx.Response().Header().Set("Content-Type", manifest.MediaType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(bz)))
	echoErr := ctx.JSONBlob(http.StatusOK, bz)
	r.logger.Log(ctx, nil)
	return echoErr
}

// PullLayer
// GET /v2/<name>/blobs/<digest>
// OK, error: binary output can mess your system ...
func (r *registry) PullLayer(ctx echo.Context) error {
	//namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ctx.Set(types.HandlerStartTime, time.Now())

	clientDigest := ctx.Param("digest")

	layer, err := r.store.GetLayer(ctx.Request().Context(), clientDigest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if layer.SkynetLink == "" {
		detail := map[string]interface{}{
			"error": "skylink is empty",
		}
		e := fmt.Errorf("skylink is empty").Error()
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, e, detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	resp, err := r.skynet.Download(layer.SkynetLink)
	if err != nil {
		detail := map[string]interface{}{
			"error":   err.Error(),
			"skylink": layer.SkynetLink,
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, resp); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = resp.Close()

	dig := digest(buf.Bytes())
	if dig != clientDigest {
		details := map[string]interface{}{
			"clientDigest":   clientDigest,
			"computedDigest": dig,
		}
		errMsg := r.errorResponse(
			RegistryErrorCodeBlobUploadUnknown,
			"client digest is different than computed digest",
			details,
		)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(buf.Bytes())))
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	echoErr := ctx.Blob(http.StatusOK, "application/octet-stream", buf.Bytes())
	r.logger.Log(ctx, nil)
	return echoErr
}

// MonolithicUpload
// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) MonolithicUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")

	if _, ok := r.b.uploads[uuid]; ok {
		errMsg := r.b.errorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			"error in monolithic upload",
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error copying request body in monolithic upload blob",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}

	_ = ctx.Request().Body.Close()
	r.b.uploads[uuid] = buf.Bytes()

	if err := r.b.blobTransaction(ctx, buf.Bytes(), uuid); err != nil {
		errMsg := r.b.errorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			err.Error(),
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)

	ctx.Response().Header().Set("Location", locationHeader)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, nil)
	return echoErr
}

// ChunkedUpload
// PATCH /v2/<name>/blobs/uploads/<uuid>
func (r *registry) ChunkedUpload(ctx echo.Context) error {
	return r.b.UploadBlob(ctx)
}

/*StartUpload
for postgres:
start a tnx
registry.tnxMap[uuid] = {txn,blobs[],timeout}
*/
// POST /v2/<name>/blobs/uploads/
func (r *registry) StartUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	clientDigest := ctx.QueryParam("digest")

	if clientDigest != "" {
		buf := &bytes.Buffer{}
		if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
			details := map[string]interface{}{
				"clientDigest": clientDigest,
				"namespace":    namespace,
			}
			errMsg := r.errorResponse(
				RegistryErrorCodeBlobUploadInvalid,
				"error while reading request body",
				details,
			)
			echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}
		_ = ctx.Request().Body.Close() // why defer? body is already read :)
		dig := digest(buf.Bytes())

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
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", fmt.Errorf("%s", errMsg)))
			return echoErr
		}

		skylink, err := r.skynet.Upload(namespace, dig, buf.Bytes(), true)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}

		id, err := uuid.NewRandom()
		if err != nil {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
				"cause": "error creating random id for layer",
			})
			r.logger.Log(ctx, err)
			return echoErr
		}
		layerV2 := &types.LayerV2{
			MediaType:   ctx.Request().Header.Get("content-type"),
			Digest:      dig,
			SkynetLink:  skylink,
			UUID:        id.String(),
			BlobDigests: nil,
			Size:        buf.Len(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		txnOp, err := r.store.NewTxn(ctx.Request().Context())
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}

		if err := r.store.SetLayer(ctx.Request().Context(), txnOp, layerV2); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}
		if err := r.store.Commit(ctx.Request().Context(), txnOp); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}

		link := r.getHttpUrlFromSkylink(skylink)
		ctx.Response().Header().Set("Location", link)
		echoErr := ctx.NoContent(http.StatusCreated)
		r.logger.Log(ctx, nil)
		return echoErr
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for blob",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, id.String())
	txn, err := r.store.NewTxn(ctx.Request().Context())
	if err != nil {
		errMsg := r.errorResponse(
			RegistryErrorCodeUnknown,
			err.Error(),
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
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
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, nil)
	return echoErr
}

//UploadProgress TODO
func (r *registry) UploadProgress(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")

	skylink, err := r.store.GetContentHashById(ctx.Request().Context(), uuid)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if skylink == "" {
		err = fmt.Errorf("skylink is empty")
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	metadata, err := r.skynet.Metadata(skylink)
	if err != nil {
		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", "bytes=0-0")
		ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
		echoErr := ctx.NoContent(http.StatusNoContent)
		r.logger.Log(ctx, err)
		return echoErr
	}

	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("bytes=0-%d", metadata.ContentLength))
	ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
	echoErr := ctx.NoContent(http.StatusNoContent)
	r.logger.Log(ctx, nil)
	return echoErr
}

// CompleteUpload
/*PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
for postgres:
this is where we insert into the layer after all the blobs have been accumulated
and inserted in the blob table
thus committing the txn
*/
func (r *registry) CompleteUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	dig := ctx.QueryParam("digest")
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	id := ctx.Param("uuid")

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = ctx.Request().Body.Close()
	// insert if bz is not nil
	ubuf := bytes.NewBuffer(r.b.uploads[id])
	ubuf.Write(buf.Bytes())
	ourHash := digest(ubuf.Bytes())
	delete(r.b.uploads, id)

	if ourHash != dig {
		details := map[string]interface{}{
			"headerDigest": dig, "serverSideDigest": ourHash, "bodyDigest": digest(buf.Bytes()),
		}
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, "digest mismatch", details)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	blobNamespace := fmt.Sprintf("%s/blobs", namespace)
	skylink, err := r.skynet.Upload(blobNamespace, dig, ubuf.Bytes(), true)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), echo.Map{
			"reason": "ERR_SKYNET_UPLOAD",
			"error":  err.Error(),
		})

		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	txnOp, ok := r.txnMap[id]
	if !ok {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, "transaction does not exist for uuid -"+id, nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	layer := &types.LayerV2{
		MediaType:   "",
		Digest:      dig,
		SkynetLink:  skylink,
		UUID:        id,
		BlobDigests: txnOp.blobDigests,
		Size:        ubuf.Len(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if !ok {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, "transaction does not exist for uuid -"+id, nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if err := r.store.SetLayer(ctx.Request().Context(), txnOp.txn, layer); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "set layer issues",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if err := r.store.Commit(ctx.Request().Context(), txnOp.txn); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "commitment issue",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	delete(r.txnMap, id)

	locationHeader := fmt.Sprintf("/v2/%s/blobs/%s", namespace, ourHash)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Content-Digest", ourHash)
	ctx.Response().Header().Set("Location", locationHeader)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, nil)
	return echoErr
}

//BlobMount to be implemented by guacamole at a later stage
func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}

//PushImage is already implemented through StartUpload and ChunkedUpload
func (r *registry) PushImage(ctx echo.Context) error {
	return nil
}

func (r *registry) PushManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")
	contentType := ctx.Request().Header.Get("Content-Type")

	var manifest ImageManifest

	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, ctx.Request().Body)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "failed in push manifest while io Copy",
		})
	}
	_ = ctx.Request().Body.Close()

	err = json.Unmarshal(buf.Bytes(), &manifest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	dig := digest(buf.Bytes())

	mfNamespace := fmt.Sprintf("%s/manifests", namespace)
	skylink, err := r.skynet.Upload(mfNamespace, dig, buf.Bytes(), true)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	var layerIDs []string
	for _, layer := range manifest.Layers {
		layerIDs = append(layerIDs, layer.Digest)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for config",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	mfc := types.ConfigV2{
		UUID:      id.String(),
		Namespace: namespace,
		Reference: ref,
		Digest:    dig,
		Skylink:   skylink,
		MediaType: contentType,
		Layers:    layerIDs,
		Size:      0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id, err = uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for image manifest",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	val := &types.ImageManifestV2{
		Uuid:          id.String(),
		Namespace:     namespace,
		MediaType:     "",
		SchemaVersion: 2,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	txnOp, err := r.store.NewTxn(context.Background())
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"reason": "PG_ERR_CREATE_NEW_TXN",
		})
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if err = r.store.SetManifest(ctx.Request().Context(), txnOp, val); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if err = r.store.SetConfig(ctx.Request().Context(), txnOp, mfc); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if err = r.store.Commit(ctx.Request().Context(), txnOp); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"reason": "ERR_PG_COMMIT_TXN",
		})
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	locationHeader := r.getHttpUrlFromSkylink(skylink)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Content-Digest", dig)
	ctx.Response().Header().Set("X-Docker-Content-ID", skylink)
	echoErr := ctx.String(http.StatusCreated, "Created")
	r.logger.Log(ctx, nil)
	return echoErr
}

// PushLayer
// POST /v2/<name>/blobs/uploads/
func (r *registry) PushLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	elem := strings.Split(ctx.Request().URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		errMsg := r.errorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for push layer",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}
	p := path.Join(elem[1 : len(elem)-2]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, id.String())
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", id.String())
	ctx.Response().Header().Set("Range", "bytes=0-0")
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, nil)
	return echoErr
}

func (r *registry) CancelUpload(ctx echo.Context) error {
	return nil
}

// DeleteTagOrManifest
// DELETE /v2/<name>/manifest/<tag> or <digest>
func (r *registry) DeleteTagOrManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	ref := ctx.Param("reference")

	if ref == "" {
		reqURI := strings.Split(ctx.Request().RequestURI, "/")
		if len(reqURI) == 6 {
			ref = reqURI[5]
		}
	}
	txnOp, _ := r.store.NewTxn(context.Background())
	if err := r.store.DeleteManifestOrTag(ctx.Request().Context(), txnOp, ref); err != nil {
		details := map[string]interface{}{
			"namespace": namespace,
			"digest":    ref,
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	err := r.store.Commit(ctx.Request().Context(), txnOp)
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, err)
	return echoErr
}

func (r *registry) DeleteLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	dig := ctx.Param("digest")
	layer, err := r.store.GetLayer(ctx.Request().Context(), dig)
	if err != nil {

		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	blobs := layer.BlobDigests

	txnOp, _ := r.store.NewTxn(context.Background())
	err = r.store.DeleteLayerV2(ctx.Request().Context(), txnOp, dig)
	if err != nil {
		logMsg := echo.Map{
			"error":  err.Error(),
			"caller": "DeleteLayer",
		}

		bz, err := json.Marshal(logMsg)
		if err == nil {
			r.logger.Log(ctx, err)
		}

		return ctx.JSONBlob(http.StatusInternalServerError, bz)
	}

	for i := range blobs {
		if err = r.store.DeleteBlobV2(ctx.Request().Context(), txnOp, blobs[i]); err != nil {
			logMsg := echo.Map{
				"error":  err.Error(),
				"caller": "DeleteLayer",
			}

			r.logger.Log(ctx, fmt.Errorf("%s", logMsg))
			bz, err := json.Marshal(logMsg)
			if err != nil {
				errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
				r.logger.Log(ctx, err)
				return ctx.JSONBlob(http.StatusBadRequest, errMsg)
			}
			return ctx.JSONBlob(http.StatusInternalServerError, bz)
		}
	}
	err = r.store.Commit(ctx.Request().Context(), txnOp)
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, err)
	return echoErr
}

// Should also look into 401 Code
// https://docs.docker.com/registry/spec/api/
func (r *registry) ApiVersion(ctx echo.Context) error {
	ctx.Response().Header().Set(HeaderDockerDistributionApiVersion, "registry/2.0")
	return ctx.String(http.StatusOK, "OK\n")
}

func (r *registry) GetImageNamespace(ctx echo.Context) error {

	searchQuery := ctx.QueryParam("search_query")
	if searchQuery == "" {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "search query must not be empty",
		})
	}
	result, err := r.store.GetImageNamespace(ctx.Request().Context(), searchQuery)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting image namespace",
		})
	}

	// empty namespace to pull full catalog list
	total, err := r.store.GetCatalogCount(ctx.Request().Context(), "")
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_GET_CATALOG_COUNT",
		})
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"repositories": result,
		"total":        total,
	})
}
