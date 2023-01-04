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
	"sync"
	"time"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	dfsImpl "github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
	"github.com/opencontainers/go-digest"
)

func NewRegistry(
	pgStore postgres.PersistentStore,
	dfs dfsImpl.DFS,
	logger telemetry.Logger,
	config *config.OpenRegistryConfig,
) (Registry, error) {
	mu := &sync.RWMutex{}
	r := &registry{
		debug:  true,
		dfs:    dfs,
		mu:     mu,
		config: config,
		b: blobs{
			contents:           make(map[string][]byte),
			uploads:            make(map[string][]byte),
			layers:             make(map[string][]string),
			blobCounter:        make(map[string]int64),
			layerLengthCounter: make(map[string]int64),
			layerParts:         make(map[string][]s3types.CompletedPart),
			mu:                 mu,
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
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return ctx.NoContent(http.StatusNotFound)
	}

	metadata, err := r.dfs.Metadata(GetManifestIdentifier(namespace, manifest.Reference))
	if err != nil {
		detail := map[string]interface{}{
			"error":   err.Error(),
			"dfsLink": manifest.DFSLink,
		}

		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if manifest.Reference != ref && manifest.Digest != ref {
		details := map[string]interface{}{
			"storedDigest": manifest.Digest,
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
	ctx.Response().WriteHeader(http.StatusOK)
	r.logger.Log(ctx, nil)
	// nil is okay here since all the required information has been set above
	return nil
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
	return fmt.Errorf("not implemented")
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
	resp, err := r.dfs.Download(ctx.Request().Context(), GetManifestIdentifier(namespace, manifest.Reference))
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
	ctx.Response().Header().Set("X-Docker-Content-ID", manifest.DFSLink)
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
	ctx.Set(types.HandlerStartTime, time.Now())

	clientDigest := ctx.Param("digest")
	layer, err := r.store.GetLayer(ctx.Request().Context(), clientDigest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if layer.DFSLink == "" {
		detail := map[string]interface{}{
			"error": "DFSLink is empty",
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, "", detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	size, err := r.dfs.Metadata(GetLayerIdentifier(layer.UUID))
	if err != nil {
		detail := map[string]interface{}{
			"error":          err.Error(),
			"operationError": "metadata service failed",
		}
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), detail)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", layer.Digest)
	ctx.Response().Header().Set("status", "307")

	url := r.getDownloadableURLFromDFSLink(layer.DFSLink)
	r.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

// MonolithicUpload
// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) MonolithicUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	imageDigest := ctx.QueryParam("digest")
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, "error while reading request body", nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = ctx.Request().Body.Close() // why defer? body is already read :)
	computedDigest := digest.FromBytes(buf.Bytes())

	if computedDigest.String() != imageDigest {
		details := map[string]interface{}{
			"clientDigest":   imageDigest,
			"computedDigest": computedDigest.String(),
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

	uuid, err := CreateIdentifier()
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	dfsLink, err := r.dfs.Upload(ctx.Request().Context(), GetLayerIdentifier(uuid), imageDigest, buf.Bytes())
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	layerV2 := &types.LayerV2{
		MediaType:   ctx.Request().Header.Get("content-type"),
		Digest:      imageDigest,
		DFSLink:     dfsLink,
		UUID:        uuid,
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

	link := r.getDownloadableURLFromDFSLink(dfsLink)
	ctx.Response().Header().Set("Location", link)
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
	imageDigest := ctx.QueryParam("digest")

	// Do a Single POST monolithic upload if the digest is present
	// reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#single-post
	if imageDigest != "" {
		return r.MonolithicUpload(ctx)
	}

	layerIdentifier, err := CreateIdentifier()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for blob",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}

	uploadId, err := r.dfs.CreateMultipartUpload(GetLayerIdentifier(layerIdentifier))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

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

	r.mu.Lock()
	r.txnMap[uploadId] = TxnStore{
		txn:         txn,
		blobDigests: []string{},
		timeout:     time.Minute * 10,
	}
	r.mu.Unlock()

	uploadTrackingID := CreateUploadTrackingIdentifier(uploadId, layerIdentifier)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uploadTrackingID)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Upload-UUID", uploadTrackingID)
	ctx.Response().Header().Set("Range", "0-0")
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, nil)
	return echoErr
}

// UploadProgress TODO
func (r *registry) UploadProgress(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	uuid := ctx.Param("uuid")
	layerkey := GetLayerIdentifierFromTrakcingID(uuid)
	uploadID := GetUploadIDFromTrakcingID(uuid)

	metadata, err := r.dfs.GetUploadProgress(GetLayerIdentifier(layerkey), uploadID)
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

func (r *registry) MonolithicPut(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	dig := ctx.QueryParam("digest")
	identifier := ctx.Param("uuid")
	layerKey := GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := GetUploadIDFromTrakcingID(identifier)

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = ctx.Request().Body.Close()
	ourHash := digest.FromBytes(buf.Bytes())

	dfsLink, err := r.dfs.Upload(ctx.Request().Context(), GetLayerIdentifier(layerKey), ourHash.String(), buf.Bytes())
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	txnOp, ok := r.txnMap[uploadID]
	if !ok {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, "transaction does not exist for uuid -"+identifier, nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	layer := &types.LayerV2{
		MediaType:   ctx.Request().Header.Get("content-type"),
		Digest:      dig,
		DFSLink:     dfsLink,
		UUID:        layerKey,
		BlobDigests: txnOp.blobDigests,
		Size:        buf.Len(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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

	downlaodableLink := r.getDownloadableURLFromDFSLink(dfsLink)
	ctx.Response().Header().Set("Docker-Content-Digest", ourHash.String())
	ctx.Response().Header().Set("Location", downlaodableLink)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, nil)
	return echoErr
}

// CompleteUpload
// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
// for postgres:
// this is where we insert into the layer after all the blobs have been accumulated
// and inserted in the blob table
// thus committing the txn
//
// NOTE - This API can also optionally receive the final blob for the upload
func (r *registry) CompleteUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	dig := ctx.QueryParam("digest")
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	identifier := ctx.Param("uuid")
	layerKey := GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := GetUploadIDFromTrakcingID(identifier)

	if r.b.blobCounter[uploadID] == 0 {
		return r.MonolithicPut(ctx)
	}

	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	_ = ctx.Request().Body.Close()
	checksum := digest.FromBytes(buf.Bytes())

	if buf.Len() > 0 {
		r.b.blobCounter[uploadID]++
		part, err := r.dfs.UploadPart(
			ctx.Request().Context(),
			uploadID,
			GetLayerIdentifier(layerKey),
			checksum.String(),
			r.b.blobCounter[uploadID],
			bytes.NewReader(buf.Bytes()),
			int64(buf.Len()),
		)
		if err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}

		r.mu.Lock()
		r.b.layerParts[uploadID] = append(r.b.layerParts[uploadID], part)
		r.mu.Unlock()
	}

	dfsLink, err := r.dfs.CompleteMultipartUpload(
		ctx.Request().Context(),
		uploadID,
		GetLayerIdentifier(layerKey),
		dig,
		r.b.layerParts[uploadID],
	)
	delete(r.b.layerParts, uploadID)

	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), echo.Map{
			"reason": "ERR_DFS_UPLOAD",
			"error":  err.Error(),
		})

		_ = r.dfs.AbortMultipartUpload(ctx.Request().Context(), GetLayerIdentifier(layerKey), uploadID)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	txnOp, ok := r.txnMap[uploadID]
	if !ok {
		errMsg := r.errorResponse(RegistryErrorCodeUnknown, "transaction does not exist for uuid -"+identifier, nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	layer := &types.LayerV2{
		MediaType:   ctx.Request().Header.Get("content-type"),
		Digest:      dig,
		DFSLink:     dfsLink,
		UUID:        layerKey,
		BlobDigests: txnOp.blobDigests,
		Size:        buf.Len(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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

	locationHeader := fmt.Sprintf("/v2/%s/blobs/%s", namespace, checksum.String())
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Content-Digest", checksum.String())
	ctx.Response().Header().Set("Location", locationHeader)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, nil)
	return echoErr
}

// BlobMount to be implemented by guacamole at a later stage
func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}

// PushImage is already implemented through StartUpload and ChunkedUpload
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

	dig := digest.FromBytes(buf.Bytes())
	dfsLink, err := r.dfs.Upload(ctx.Request().Context(), GetManifestIdentifier(namespace, ref), dig.String(), buf.Bytes())
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

	uuid, err := CreateIdentifier()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for config",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}

	mfc := types.ConfigV2{
		UUID:      uuid,
		Namespace: namespace,
		Reference: ref,
		Digest:    dig.String(),
		DFSLink:   dfsLink,
		MediaType: contentType,
		Layers:    layerIDs,
		Size:      0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	val := &types.ImageManifestV2{
		Uuid:          uuid,
		Namespace:     namespace,
		MediaType:     contentType,
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

	locationHeader := fmt.Sprintf("https://openregsitry-test.s3.amazonaws.com/%s", dfsLink)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Content-Digest", dig.String())
	ctx.Response().Header().Set("X-Docker-Content-ID", dfsLink)
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

	uuid, err := CreateIdentifier()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for push layer",
		})
		r.logger.Log(ctx, err)
		return echoErr
	}

	p := path.Join(elem[1 : len(elem)-2]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
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
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	for i := range blobs {
		if err = r.store.DeleteBlobV2(ctx.Request().Context(), txnOp, blobs[i]); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
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
