package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/opencontainers/go-digest"
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
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return ctx.NoContent(http.StatusNotFound)
	}

	metadata, err := b.registry.dfs.Metadata(GetLayerIdentifier(layerRef.UUID))
	if err != nil {
		details := echo.Map{
			"error":   err.Error(),
			"message": "DFS - Metadata not found for: " + layerRef.DFSLink,
		}
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", details)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return ctx.NoContent(http.StatusNotFound)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", metadata.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	err = ctx.String(http.StatusOK, "OK")
	b.registry.logger.Log(ctx, nil)
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

	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	contentRange := ctx.Request().Header.Get("Content-Range")
	identifier := ctx.Param("uuid")
	layerKey := GetLayerIdentifierFromTrakcingID(identifier)
	uploadID := GetUploadIDFromTrakcingID(identifier)

	if contentRange == "" {
		// if _, ok := b.uploads[uuid]; ok {
		// 	errMsg := b.errorResponse(
		// 		RegistryErrorCodeBlobUploadInvalid,
		// 		"stream upload after first write are not allowed",
		// 		nil,
		// 	)
		// 	echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		// 	b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		// 	return echoErr
		// }

		// buf := &bytes.Buffer{}
		// if _, err := io.Copy(buf, ctx.Request().Body); err != nil {
		// 	echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
		// 		"error":   err.Error(),
		// 		"message": "error copying request body in upload blob",
		// 	})
		// 	b.registry.logger.Log(ctx, err)
		// 	return echoErr
		// }

		// _ = ctx.Request().Body.Close()
		// b.uploads[uuid] = buf.Bytes()

		// if err := b.blobTransaction(ctx, buf.Bytes(), uuid); err != nil {
		// 	errMsg := b.errorResponse(
		// 		RegistryErrorCodeBlobUploadInvalid,
		// 		err.Error(),
		// 		nil,
		// 	)
		// 	echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		// 	b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		// 	return echoErr
		// }

		buf := &bytes.Buffer{}
		_, err := io.Copy(buf, ctx.Request().Body)
		if err != nil {
			fmt.Println(err)
		}
		ctx.Request().Body.Close()

		checksum := digest.FromBytes(buf.Bytes())

		b.blobCounter[uploadID]++
		part, err := b.registry.dfs.UploadPart(
			ctx.Request().Context(),
			uploadID,
			GetLayerIdentifier(layerKey),
			checksum.String(),
			b.blobCounter[uploadID],
			bytes.NewReader(buf.Bytes()),
			int64(buf.Len()),
		)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error":   err.Error(),
				"message": "error uploading blob",
			})
			b.registry.logger.Log(ctx, err)
			return echoErr
		}
		b.mu.Lock()
		b.layerParts[uploadID] = append(b.layerParts[uploadID], part)
		b.mu.Unlock()
		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, identifier)
		ctx.Response().Header().Set("Location", locationHeader)
		b.layerLengthCounter[uploadID] = int64(buf.Len())
		ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", b.layerLengthCounter[uploadID]-1))
		err = ctx.NoContent(http.StatusAccepted)
		b.registry.logger.Log(ctx, nil)
		return err
	}

	var start, end int64
	// 0-90
	if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
		details := map[string]interface{}{
			"error":        err.Error(),
			"message":      "content range is invalid",
			"contentRange": contentRange,
		}
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), details)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	if start != b.layerLengthCounter[uploadID] {
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content range mismatch", nil)
		echoErr := ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}

	// buf := bytes.NewBuffer(b.uploads[uuid]) // 90
	// _, err := io.Copy(buf, ctx.Request().Body)
	// if err != nil {
	// 	errMsg := b.errorResponse(
	// 		RegistryErrorCodeBlobUploadInvalid,
	// 		"error while creating new buffer from existing blobs",
	// 		nil,
	// 	)
	// 	echoErr := ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	// 	b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
	// 	return echoErr
	// } // 10
	// ctx.Request().Body.Close()

	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, ctx.Request().Body)
	if err != nil {
		fmt.Println(err)
	}

	checksum := digest.FromBytes(buf.Bytes())
	b.blobCounter[uploadID]++
	part, err := b.registry.dfs.UploadPart(
		ctx.Request().Context(),
		uploadID,
		GetLayerIdentifier(layerKey),
		checksum.String(),
		b.blobCounter[uploadID],
		bytes.NewReader(buf.Bytes()),
		int64(buf.Len()),
	)
	defer ctx.Request().Body.Close()
	if err != nil {
		errMsg := b.errorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			err.Error(),
			nil,
		)
		echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
		b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
		return echoErr
	}
	b.mu.Lock()
	b.layerParts[uploadID] = append(b.layerParts[uploadID], part)
	b.layerLengthCounter[uploadID] += int64(buf.Len())
	b.mu.Unlock()

	// b.uploads[uuid] = buf.Bytes()
	// if err := b.blobTransaction(ctx, buf.Bytes(), uuid); err != nil {
	// 	errMsg := b.errorResponse(
	// 		RegistryErrorCodeBlobUploadInvalid,
	// 		err.Error(),
	// 		nil,
	// 	)
	// 	echoErr := ctx.JSONBlob(http.StatusBadRequest, errMsg)
	// 	b.registry.logger.Log(ctx, fmt.Errorf("%s", errMsg))
	// 	return echoErr
	// }
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, identifier)
	ctx.Response().Header().Set("Location", locationHeader)
	// ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", buf.Len()-1))
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", b.layerLengthCounter[uploadID]-1))
	echoErr := ctx.NoContent(http.StatusAccepted)
	b.registry.logger.Log(ctx, nil)
	return echoErr
}

func (b *blobs) blobTransaction(ctx echo.Context, bz []byte, uuid string) error {
	checksum := digest.FromBytes(bz)
	blob := &types.Blob{
		Digest:     checksum.String(),
		Skylink:    "",
		UUID:       uuid,
		RangeStart: 0,
		RangeEnd:   uint32(len(bz) - 1),
	}

	txnOp, ok := b.registry.txnMap[uuid]
	if !ok {
		return fmt.Errorf("txn has not been initialised for uuid - " + uuid)
	}

	if err := b.registry.store.SetBlob(ctx.Request().Context(), txnOp.txn, blob); err != nil {
		color.Red("aborting txn: %s\n", err.Error())
		return b.registry.store.Abort(ctx.Request().Context(), txnOp.txn)
	}

	txnOp.blobDigests = append(txnOp.blobDigests, blob.Digest)
	b.registry.txnMap[uuid] = txnOp
	return nil
}
