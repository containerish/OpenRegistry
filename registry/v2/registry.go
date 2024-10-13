package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	oci_digest "github.com/opencontainers/go-digest"
	img_spec_v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/config"
	dfsImpl "github.com/containerish/OpenRegistry/dfs"
	store_v2 "github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
)

func NewRegistry(
	pgStore store_v2.RegistryStore,
	dfs dfsImpl.DFS,
	logger telemetry.Logger,
	config *config.OpenRegistryConfig,
) Registry {
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
			blobCounter:        make(map[string]int32),
			layerLengthCounter: make(map[string]int),
			layerParts:         make(map[string][]s3types.CompletedPart),
			mu:                 mu,
		},
		logger: logger,
		store:  pgStore,
		txnMap: map[string]TxnStore{},
	}

	r.b.registry = r

	return r
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

// References:
// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#checking-if-content-exists-in-the-registry
// HEAD /v2/<name>/manifests/<reference>
func (r *registry) ManifestExists(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	ref := ctx.Param("reference") // ref can be either tag or digest

	manifest, err := r.store.GetManifestByReference(ctx.Request().Context(), namespace, ref)
	if err != nil {
		details := echo.Map{
			"error":     err.Error(),
			"message":   "manifest not found",
			"reference": ref,
		}

		errMsg := common.RegistryErrorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, errMsg).Send()
		return echoErr
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", manifest.Size))
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	ctx.Response().Header().Set("Content-Type", img_spec_v1.MediaTypeImageManifest)
	echoErr := ctx.NoContent(http.StatusOK)
	r.logger.Log(ctx, nil).Any("manifest", manifest).Send()
	// nil is okay here since all the required information has been set above
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
	var pageSize int
	var offset int
	if queryParamPageSize != "" {
		ps, err := strconv.Atoi(ctx.QueryParam("n"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			r.logger.Log(ctx, err).Send()
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.Atoi(ctx.QueryParam("last"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			r.logger.Log(ctx, err).Send()
			return echoErr
		}
		offset = int(o)
	}

	catalogList, err := r.store.GetCatalog(ctx.Request().Context(), namespace, pageSize, offset)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}
	// empty namespace to pull the full catalog list
	total, err := r.store.GetCatalogCount(ctx.Request().Context(), "")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}
	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": catalogList,
		"total":        total,
	})
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

// ListTags Content discovery
// GET /v2/<name>/tags/list
// OK
func (r *registry) ListTags(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	limit := ctx.QueryParam("n")

	tags, err := r.store.GetImageTags(ctx.Request().Context(), namespace)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if limit != "" {
		n, err := strconv.ParseInt(limit, 10, 32)
		if err != nil {
			errMsg := common.RegistryErrorResponse(RegistryErrorCodeTagInvalid, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
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
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (r *registry) List(ctx echo.Context) error {
	return fmt.Errorf("not implemented")
}

// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
// GET /v2/<name>/manifests/<reference>
func (r *registry) PullManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	ref := ctx.Param("reference")

	if strings.HasPrefix(ref, "sha256:") {
		if _, err := oci_digest.Parse(ref); err != nil {
			errMsg := common.RegistryErrorResponse(RegistryErrorCodeDigestInvalid, err.Error(), echo.Map{
				"namespace": namespace,
				"ref":       ref,
			})
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
			return echoErr
		}
	}

	manifest, err := r.store.GetManifestByReference(ctx.Request().Context(), namespace, ref)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeManifestUnknown, "manifest not found", echo.Map{
			"error":     err.Error(),
			"namespace": namespace,
			"ref":       ref,
		})
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	defer func() {
		err = r.store.IncrementRepositoryPullCounter(ctx.Request().Context(), manifest.RepositoryID)
		// silently fail
		if err != nil {
			r.logger.DebugWithContext(ctx).Err(err).Send()
		}
	}()

	trimmedMf := manifest.ToOCISubject()
	ctx.Response().Header().Set("Docker-Content-Digest", manifest.Digest)
	ctx.Response().Header().Set("Content-Type", manifest.MediaType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", len(trimmedMf)))
	echoErr := ctx.JSONBlob(http.StatusOK, trimmedMf)
	r.logger.Log(ctx, nil).Send()
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
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if layer.DFSLink == "" {
		detail := map[string]interface{}{
			"error": "DFSLink is empty",
		}
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, "", detail)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	size, err := r.dfs.Metadata(layer)
	if err != nil {
		detail := map[string]interface{}{
			"error":          err.Error(),
			"operationError": "metadata service failed",
		}
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), detail)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", layer.Digest)
	ctx.Response().Header().Set("status", "307")

	downloadableURL, err := r.getDownloadableURLFromDFSLink(layer.DFSLink)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	echoErr := ctx.Redirect(http.StatusTemporaryRedirect, downloadableURL)
	r.logger.Log(ctx, nil).Str("redirect_uri", downloadableURL).Send()
	return echoErr
}

// MonolithicUpload
// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) MonolithicUpload(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	imageDigest := ctx.QueryParam("digest")
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := common.RegistryErrorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			"error while reading request body",
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()
	computedDigest := oci_digest.FromBytes(buf.Bytes())

	if computedDigest.String() != imageDigest {
		details := map[string]interface{}{
			"clientDigest":   imageDigest,
			"computedDigest": computedDigest.String(),
		}
		errMsg := common.RegistryErrorResponse(
			RegistryErrorCodeDigestInvalid,
			"client digest does not meet computed digest",
			details,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", fmt.Errorf("%s", errMsg))).Send()
		return echoErr
	}

	uuid, err := types.CreateIdentifier()
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	dfsLink, err := r.dfs.Upload(ctx.Request().Context(), types.GetLayerIdentifier(uuid), imageDigest, buf.Bytes())
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg.Bytes())
		r.logger.Log(ctx, errMsg).Send()
		return echoErr
	}

	layerV2 := &types.ContainerImageLayer{
		MediaType: ctx.Request().Header.Get("content-type"),
		Digest:    imageDigest,
		DFSLink:   dfsLink,
		ID:        uuid,
		Size:      int64(buf.Len()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	txnOp, err := r.store.NewTxn(ctx.Request().Context())
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err = r.store.SetLayer(ctx.Request().Context(), txnOp, layerV2); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err = r.store.Commit(ctx.Request().Context(), txnOp); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	downloadableURL, err := r.getDownloadableURLFromDFSLink(dfsLink)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	ctx.Response().Header().Set("Location", downloadableURL)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, echoErr).Send()
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

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	imageDigest := ctx.QueryParam("digest")

	// Do a Single POST monolithic upload if the digest is present
	// reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#single-post
	if imageDigest != "" {
		return r.MonolithicUpload(ctx)
	}

	layerIdentifier, err := types.CreateIdentifier()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for blob",
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	uploadId, err := r.dfs.CreateMultipartUpload(types.GetLayerIdentifier(layerIdentifier))
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	txn, err := r.store.NewTxn(context.Background())
	if err != nil {
		errMsg := common.RegistryErrorResponse(
			RegistryErrorCodeUnknown,
			err.Error(),
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	r.mu.Lock()
	r.txnMap[uploadId] = TxnStore{
		txn:         txn,
		blobDigests: []string{},
		timeout:     time.Minute * 10,
	}
	r.mu.Unlock()

	uploadTrackingID := types.CreateUploadTrackingIdentifier(uploadId, layerIdentifier)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uploadTrackingID)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Upload-UUID", uploadTrackingID)
	ctx.Response().Header().Set("OCI-Chunk-Min-Length", fmt.Sprintf("%d", r.dfs.Config().MinChunkSize))
	ctx.Response().Header().Set("Range", "0-0")
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

// UploadProgress TODO
func (r *registry) UploadProgress(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	uuid := ctx.Param("uuid")
	layerkey := types.GetLayerIdentifierFromTrakcingID(uuid)
	uploadID := types.GetUploadIDFromTrakcingID(uuid)

	metadata, err := r.dfs.GetUploadProgress(types.GetLayerIdentifier(layerkey), uploadID)
	if err != nil {
		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", "0-0")
		ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
		echoErr := ctx.NoContent(http.StatusNoContent)
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", metadata.ContentLength-1))
	ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
	echoErr := ctx.NoContent(http.StatusNoContent)
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (r *registry) MonolithicPut(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	digest := ctx.QueryParam("digest")
	identifier := ctx.Param("uuid")
	layerKey := types.GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := types.GetUploadIDFromTrakcingID(identifier)

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()
	ourHash := oci_digest.FromBytes(buf.Bytes())

	dfsLink, err := r.dfs.Upload(
		ctx.Request().Context(),
		types.GetLayerIdentifier(layerKey),
		ourHash.String(),
		buf.Bytes(),
	)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	txnOp, ok := r.txnMap[uploadID]
	if !ok {
		errMsg := common.RegistryErrorResponse(
			RegistryErrorCodeUnknown,
			"transaction does not exist for uuid -"+identifier,
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	layer := &types.ContainerImageLayer{
		CreatedAt: time.Now(),
		ID:        layerKey,
		Digest:    digest,
		MediaType: ctx.Request().Header.Get("content-type"),
		DFSLink:   dfsLink,
		Size:      int64(buf.Len()),
	}

	if err = r.store.SetLayer(ctx.Request().Context(), txnOp.txn, layer); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "set layer issues",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err = r.store.Commit(ctx.Request().Context(), txnOp.txn); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "commitment issue",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	downlaodableURL, err := r.getDownloadableURLFromDFSLink(dfsLink)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}
	ctx.Response().Header().Set("Docker-Content-Digest", ourHash.String())
	ctx.Response().Header().Set("Location", downlaodableURL)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, echoErr).Send()
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

	digest := ctx.QueryParam("digest")
	namespace := ctx.Get(string(RegistryNamespace)).(string)
	identifier := ctx.Param("uuid")
	layerKey := types.GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := types.GetUploadIDFromTrakcingID(identifier)

	if r.b.blobCounter[uploadID] == 0 {
		return r.MonolithicPut(ctx)
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()
	checksum := oci_digest.FromBytes(buf.Bytes())

	if buf.Len() > 0 {
		r.b.blobCounter[uploadID]++
		part, err := r.dfs.UploadPart(
			ctx.Request().Context(),
			uploadID,
			types.GetLayerIdentifier(layerKey),
			checksum.String(),
			r.b.blobCounter[uploadID],
			bytes.NewReader(buf.Bytes()),
			int64(buf.Len()),
		)
		if err != nil {
			errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
			return echoErr
		}

		r.mu.Lock()
		r.b.layerParts[uploadID] = append(r.b.layerParts[uploadID], part)
		r.b.layerLengthCounter[uploadID] += buf.Len()
		r.mu.Unlock()
	}

	dfsLink, err := r.dfs.CompleteMultipartUpload(
		ctx.Request().Context(),
		uploadID,
		types.GetLayerIdentifier(layerKey),
		digest,
		r.b.layerParts[uploadID],
	)
	r.mu.Lock()
	delete(r.b.layerParts, uploadID)
	r.mu.Unlock()

	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), echo.Map{
			"reason": "ERR_DFS_COMPLETE_MULTI_PART_UPLOAD",
			"error":  err.Error(),
		})

		_ = r.dfs.AbortMultipartUpload(ctx.Request().Context(), types.GetLayerIdentifier(layerKey), uploadID)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	txnOp, ok := r.txnMap[uploadID]
	if !ok {
		errMsg := common.RegistryErrorResponse(
			RegistryErrorCodeUnknown,
			"transaction does not exist for uuid -"+identifier,
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	layer := &types.ContainerImageLayer{
		MediaType: ctx.Request().Header.Get("content-type"),
		Digest:    digest,
		DFSLink:   dfsLink,
		ID:        layerKey,
		Size:      int64(r.b.layerLengthCounter[uploadID]),
		CreatedAt: time.Now(),
	}

	if err := r.store.SetLayer(ctx.Request().Context(), txnOp.txn, layer); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "set layer issues",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err := r.store.Commit(ctx.Request().Context(), txnOp.txn); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"error_detail": "commitment issue",
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	locationHeader := fmt.Sprintf("/v2/%s/blobs/%s", namespace, checksum.String())
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Content-Digest", checksum.String())
	ctx.Response().Header().Set("Location", locationHeader)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, echoErr).Send()
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

// References:
// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-manifests
// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-manifests-with-subject
// - https://github.com/opencontainers/image-spec/blob/main/manifest.md#image-manifest-property-descriptions
// Method: PUT
// Path: /v2/<namespace>/manifests/<reference>
func (r *registry) PushManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	ref := ctx.Param("reference")

	user, err := r.GetUserFromCtx(ctx)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"message": "Unauthorized",
		})

		echoErr := ctx.JSONBlob(http.StatusUnauthorized, errMsg.Bytes())
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	repository := r.GetRepositoryFromCtx(ctx)
	repositoryExists := r.store.RepositoryExists(ctx.Request().Context(), namespace)
	if repository == nil || !repositoryExists {
		repositoryID, idErr := types.NewUUID()
		if idErr != nil {
			errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, idErr.Error(), echo.Map{
				"reason": "ERR_CREATE_UNIQUE_REPOSITORY_IDENTIFIER",
			})
			echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
			r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
			return echoErr

		}

		repository = &types.ContainerImageRepository{
			CreatedAt:  time.Now(),
			OwnerID:    user.ID,
			ID:         repositoryID,
			Name:       strings.Split(namespace, "/")[1],
			Visibility: types.RepositoryVisibilityPrivate,
		}

		// IPFS P2P repositories are public
		if user.Username == types.SystemUsernameIPFS {
			repository.Visibility = types.RepositoryVisibilityPublic
		}

		idErr = r.store.CreateRepository(ctx.Request().Context(), repository)
		if idErr != nil {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   idErr.Error(),
				"message": "error creating new repository",
			})
			r.logger.Log(ctx, idErr).Send()
			return echoErr
		}
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, ctx.Request().Body)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "failed in push manifest while io Copy",
		})

		r.logger.Log(ctx, nil).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	digest := oci_digest.FromBytes(buf.Bytes())

	uuid, err := types.NewUUID()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating random id for config",
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	manifest := types.ImageManifest{
		CreatedAt:    time.Now(),
		ID:           uuid,
		RepositoryID: repository.ID,
		OwnerID:      user.ID,
		Digest:       digest.String(),
		Reference:    ref,
		Size:         0,
	}

	if err = json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if manifest.MediaType == "" {
		manifest.MediaType = img_spec_v1.MediaTypeImageManifest
	}

	var layerIDs []string

	for _, layer := range manifest.Layers {
		layerIDs = append(layerIDs, layer.Digest.String())
	}

	size, err := r.store.GetImageSizeByLayerIds(ctx.Request().Context(), layerIDs)
	if err == nil {
		manifest.Size = size
	}

	txnOp, err := r.store.NewTxn(context.Background())
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"reason": "PG_ERR_CREATE_NEW_TXN",
		})
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err = r.store.SetManifest(ctx.Request().Context(), txnOp, &manifest); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"message": "invalid input provided",
		})
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if err = r.store.Commit(ctx.Request().Context(), txnOp); err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeUnknown, err.Error(), echo.Map{
			"reason": "ERR_PG_COMMIT_TXN",
		})
		_ = r.store.Abort(ctx.Request().Context(), txnOp)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	r.setPushManifestHaeders(ctx, namespace, ref, digest.String(), &manifest)
	echoErr := ctx.NoContent(http.StatusCreated)
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (r *registry) setPushManifestHaeders(
	ctx echo.Context,
	namespace, ref, digest string,
	manifest *types.ImageManifest,
) {
	locationHeader := fmt.Sprintf("%s/v2/%s/manifests/%s", r.config.Endpoint(), namespace, ref)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	ctx.Response().Header().Set("Content-Type", manifest.MediaType)

	// set OCI-Subject header if we get a manifest with a subject
	if manifest.Subject != nil {
		ctx.Response().Header().Set("OCI-Subject", manifest.Subject.Digest.String())
	}
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
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	uuid, err := types.CreateIdentifier()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for push layer",
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	p := path.Join(elem[1 : len(elem)-2]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", uuid)
	ctx.Response().Header().Set("Range", "0-0")
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (r *registry) CancelUpload(ctx echo.Context) error {
	return nil
}

// DeleteTagOrManifest
// DELETE /v2/<name>/manifest/<tag> or <digest>
func (r *registry) DeleteTagOrManifest(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	ref := ctx.Param("reference")

	if ref == "" {
		reqURI := strings.Split(ctx.Request().RequestURI, "/")
		if len(reqURI) == 6 {
			ref = reqURI[5]
		}
	}
	if err := r.store.DeleteManifestOrTag(ctx.Request().Context(), ref); err != nil {
		details := map[string]interface{}{
			"namespace": namespace,
			"digest":    ref,
		}
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeManifestUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusNotFound, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (r *registry) DeleteLayer(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	digest := ctx.Param("digest")

	txnOp, _ := r.store.NewTxn(context.Background())
	err := r.store.DeleteLayerByDigestWithTxn(ctx.Request().Context(), txnOp, digest)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	err = r.store.Commit(ctx.Request().Context(), txnOp)
	echoErr := ctx.NoContent(http.StatusAccepted)
	r.logger.Log(ctx, err).Send()
	return echoErr
}

// Should also look into 401 Code
// https://docs.docker.com/registry/spec/api/
func (r *registry) ApiVersion(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	ctx.Response().Header().Set(HeaderDockerDistributionApiVersion, "registry/2.0")
	echoErr := ctx.String(http.StatusOK, "OK\n")
	r.logger.Log(ctx, nil).Send()
	return echoErr
}

func (r *registry) GetImageNamespace(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	userId := uuid.NullUUID{}.UUID
	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if ok {
		userId = user.ID
	}

	searchQuery := ctx.QueryParam("search_query")
	if searchQuery == "" {
		errMsg := fmt.Errorf("search query must not be empty")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": errMsg.Error(),
		})

		r.logger.Log(ctx, errMsg).Send()
		return echoErr
	}

	var visibility types.RepositoryVisibility
	if ctx.QueryParam("public") == "true" {
		visibility = types.RepositoryVisibilityPublic
	}

	result, err := r.store.GetImageNamespace(ctx.Request().Context(), searchQuery, visibility, userId)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error getting image namespace",
		})

		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	// empty namespace to pull full catalog list
	total, err := r.store.GetCatalogCount(ctx.Request().Context(), "")
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "ERR_GET_CATALOG_COUNT",
		})
		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": result,
		"total":        total,
	})
	r.logger.Log(ctx, nil).Send()
	return echoErr
}

func (r *registry) GetUserFromCtx(ctx echo.Context) (*types.User, error) {
	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		return nil, fmt.Errorf("ERR_USER_NOT_PRESENT_IN_REQUEST_CTX")
	}

	return user, nil
}

func (r *registry) GetRepositoryFromCtx(ctx echo.Context) *types.ContainerImageRepository {
	if repository, ok := ctx.Get(string(types.UserRepositoryContextKey)).(*types.ContainerImageRepository); ok {
		return repository
	}
	return nil
}

// Reference:
// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#enabling-the-referrers-api
// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers
func (r *registry) ListReferrers(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	digest := ctx.Param("digest")
	artifactType := ctx.QueryParams().Get("artifactType")

	_, err := oci_digest.Parse(digest)
	if err != nil {
		errMsg := common.RegistryErrorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg.Bytes())
		r.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	var filterValues []string
	if artifactType != "" {
		parts := strings.Split(artifactType, ",")
		for _, p := range parts {
			if filter, decodeErr := url.QueryUnescape(p); decodeErr == nil {
				filterValues = append(filterValues, filter)
			}
		}
		ctx.Response().Header().Set("OCI-Filters-Applied", "artifactType")
	}

	ctx.Response().Header().Set("content-type", img_spec_v1.MediaTypeImageIndex)
	refIndex, err := r.store.GetReferrers(ctx.Request().Context(), namespace, digest, filterValues)
	if err != nil {
		echoErr := ctx.JSON(http.StatusOK, refIndex)
		r.logger.Log(ctx, err).
			Bool("referrersFound", false).
			Str("artifactType", artifactType).
			Str("digest", digest).
			Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, refIndex)
	r.logger.Log(ctx, nil).
		Bool("referrersFound", true).
		Str("artifactType", artifactType).
		Str("digest", digest).
		Send()
	return echoErr
}
