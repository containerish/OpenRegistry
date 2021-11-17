package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
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
	digest := ctx.Param("digest")
	layerRef, err := b.registry.localCache.GetDigest(digest)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
		}
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)

		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	size, ok := b.registry.skynet.Metadata(layerRef.Skylink)
	if !ok {
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	return ctx.String(http.StatusOK, "OK")
}

func (b *blobs) UploadBlob(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	contentRange := ctx.Request().Header.Get("Content-Range")
	uuid := ctx.Param("uuid")

	if contentRange == "" {
		if _, ok := b.uploads[uuid]; ok {
			errMsg := b.errorResponse(
				RegistryErrorCodeBlobUploadInvalid,
				"stream upload after first write are not allowed",
				nil,
			)
			ctx.Set(types.HttpEndpointErrorKey, errMsg)
			return ctx.JSONBlob(http.StatusBadRequest, errMsg)
		}

		bz, _ := io.ReadAll(ctx.Request().Body)
		defer ctx.Request().Body.Close()

		b.uploads[uuid] = bz

		locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
		ctx.Response().Header().Set("Location", locationHeader)
		ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", len(bz)-1))
		return ctx.NoContent(http.StatusAccepted)
	}

	start, end := 0, 0
	// 0-90
	if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
		details := map[string]interface{}{
			"error":        "content range is invalid",
			"contentRange": contentRange,
		}
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), details)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	if start != len(b.uploads[uuid]) {
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content range mismatch", nil)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	buf := bytes.NewBuffer(b.uploads[uuid]) // 90
	_, err := io.Copy(buf, ctx.Request().Body)
	if err != nil {
		errMsg := b.errorResponse(
			RegistryErrorCodeBlobUploadInvalid,
			"error while creating new buffer from existing blobs",
			nil,
		)
		ctx.Set(types.HttpEndpointErrorKey, errMsg)
		return ctx.JSONBlob(http.StatusInternalServerError, errMsg)
	} // 10
	ctx.Request().Body.Close()

	b.uploads[uuid] = buf.Bytes()
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", buf.Len()-1))
	return ctx.NoContent(http.StatusAccepted)
}
