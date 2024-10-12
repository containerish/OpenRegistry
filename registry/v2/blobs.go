package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	oci_digest "github.com/opencontainers/go-digest"

	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (b *blobs) errorResponse(code, msg string, detail map[string]interface{}) []byte {
	var err RegistryErrors

	err.Errors = append(err.Errors, RegistryError{
		Code:    code,
		Message: msg,
		Detail:  detail,
	})

	bz, e := json.Marshal(err)
	if e != nil {
		color.Red("blob error: %s", e.Error())
		return []byte{}
	}

	return bz
}

func (b *blobs) HEAD(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	digest := ctx.Param("digest")

	layerRef, err := b.registry.store.GetLayer(ctx.Request().Context(), digest)
	if err != nil {
		details := echo.Map{
			"error":   err.Error(),
			"message": "DFS: layer not found",
		}
		errMsg := b.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), details)
		echoErr := ctx.NoContent(http.StatusNotFound)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	metadata, err := b.registry.dfs.Metadata(layerRef)
	if err != nil {
		details := echo.Map{
			"error":   err.Error(),
			"message": "DFS - Metadata not found for: " + layerRef.DFSLink,
		}
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", details)
		echoErr := ctx.NoContent(http.StatusNotFound)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", metadata.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	err = ctx.String(http.StatusOK, "OK")
	b.registry.logger.Log(ctx, err).Send()
	return err
}

/*
UploadBlob
for postgres
insert into blob table one blob at a time
these will be part of the txn in StartUpload
*/
func (b *blobs) UploadBlob(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	namespace := ctx.Get(string(RegistryNamespace)).(string)
	contentRange := ctx.Request().Header.Get("Content-Range")
	identifier := ctx.Param("uuid")
	layerKey := types.GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := types.GetUploadIDFromTrakcingID(identifier)

	// upload the first chunk for the layer
	if contentRange == "" || strings.HasPrefix(contentRange, "0-") {
		if len(b.layerParts[uploadID]) > 0 {
			errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content range mismatch", nil)
			echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
			b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
			return echoErr
		}
		buf := &bytes.Buffer{}
		_, err := io.Copy(buf, ctx.Request().Body)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "error copying body to buffer",
			})
			b.registry.logger.Log(ctx, err).Send()
			return echoErr
		}
		defer ctx.Request().Body.Close()

		digest := oci_digest.FromBytes(buf.Bytes())

		b.blobCounter[uploadID]++
		part, err := b.registry.dfs.UploadPart(
			ctx.Request().Context(),
			uploadID,
			types.GetLayerIdentifier(layerKey),
			digest.String(),
			b.blobCounter[uploadID],
			bytes.NewReader(buf.Bytes()),
			int64(buf.Len()),
		)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "error uploading blob",
			})
			b.registry.logger.Log(ctx, err).Send()
			return echoErr
		}

		b.mu.Lock()
		b.layerParts[uploadID] = append(b.layerParts[uploadID], part)
		b.layerLengthCounter[uploadID] = buf.Len()
		b.mu.Unlock()

		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, identifier)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", b.layerLengthCounter[uploadID]-1))
		err = ctx.NoContent(http.StatusAccepted)
		b.registry.logger.Log(ctx, err).Send()
		return err
	}

	// continue with rest of the chunks for the layer
	var start, end int
	// 0-90
	if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
		details := map[string]interface{}{
			"error":        err.Error(),
			"message":      "content range is invalid",
			"contentRange": contentRange,
		}
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	if start != b.layerLengthCounter[uploadID] {
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content range mismatch", nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, ctx.Request().Body)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error copying body to buffer",
		})
		b.registry.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	digest := oci_digest.FromBytes(buf.Bytes())
	b.blobCounter[uploadID]++
	part, err := b.registry.dfs.UploadPart(
		ctx.Request().Context(),
		uploadID,
		types.GetLayerIdentifier(layerKey),
		digest.String(),
		b.blobCounter[uploadID],
		bytes.NewReader(buf.Bytes()),
		int64(buf.Len()),
	)
	if err != nil {
		errMsg := b.errorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			err.Error(),
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg)).Send()
		return echoErr
	}

	b.mu.Lock()
	b.layerParts[uploadID] = append(b.layerParts[uploadID], part)
	b.layerLengthCounter[uploadID] += buf.Len()
	b.mu.Unlock()
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, identifier)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", b.layerLengthCounter[uploadID]-1))
	echoErr := ctx.NoContent(http.StatusAccepted)
	b.registry.logger.Log(ctx, echoErr).Send()
	return echoErr
}
