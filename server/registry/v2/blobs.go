package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
		color.Red("bloberror: %s", e.Error())
		return []byte{}
	}

	return bz
}

func (b *blobs) get(namespace string) (map[string][]byte, bool) {
	digests, ok := b.layers[namespace]
	if !ok {
		return nil, false
	}

	layers := make(map[string][]byte)
	for _, d := range digests {
		blob, ok := b.contents[d]
		if !ok {
			return nil, false
		}
		layers[d] = blob
	}

	return layers, true
}

func (b *blobs) remove(repo string) {
	digests, ok := b.layers[repo]
	if !ok {
		return
	}
	delete(b.layers, repo)

	for _, d := range digests {
		delete(b.contents, d)
	}
}

func (b *blobs) PrintContents(ctx echo.Context) error {
	l := 0

	for _,v := range b.contents {
		l = l + len(v)
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"bufLength": l,
	})
}

func (b *blobs) HEAD(ctx echo.Context) error {

	// namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	digest := ctx.Param("digest")

	// content is available if image is locally pushed
	// if c, ok := b.contents[digest]; ok {
	// 	ctx.Response().Header().Set("Content-Length", fmt.Sprint(len(c)))
	// 	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	// 	return ctx.NoContent(http.StatusOK)
	// }

	layerRef, err := b.registry.localCache.GetDigest(digest)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
		}
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	size, ok := b.registry.skynet.Metadata(layerRef.Skylink)
	if !ok {
		errMsg := b.errorResponse(RegistryErrorCodeManifestBlobUnknown, "Manifest does not exist", nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	// bz, err := b.registry.localCache.Get([]byte(namespace))
	// if err != nil {
	// 	errMsg := b.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
	// 	return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// }

	// var md types.Metadata
	// if err = json.Unmarshal(bz, &md); err != nil {
	// 	errMsg := b.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
	// 	return ctx.JSONBlob(http.StatusNotFound, errMsg)
	// }

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", size))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	// io.CopyN(ctx.Response(), bytes.NewReader(bz), int64(size))
	return ctx.String(http.StatusOK, "OK")
}

func (b *blobs) UploadBlob(ctx echo.Context) error {
	namespace := ctx.Param("username") + "/" + ctx.Param("imagename")
	// uuid := ctx.Param("buggu")
	contentRange := ctx.Request().Header.Get("Content-Range")
	// if uuid == "" || uuid == " " {
	// 	parts := strings.Split(ctx.Request().RequestURI, "/")
	// 	color.Green("%v", parts)
	// 		uuid = parts[6]
	// }

	uuid := strings.Split(ctx.Request().RequestURI, "/")[6]
	// color.Magenta(strings.Split(ctx.Request().RequestURI, "/")[6])

	if contentRange == "" {
		if _, ok := b.uploads[uuid]; ok {
			errMsg := b.errorResponse(RegistryErrorCodeBlobUploadInvalid, "stream upload after first write are not allowed", nil)
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
			"error":       "content range is invalid",
			"conentRange": contentRange,
		}
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), details)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	if start != len(b.uploads[uuid]) {
		errMsg := b.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content range mismatch", nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	buf := bytes.NewBuffer(b.uploads[uuid]) // 90
	io.Copy(buf, ctx.Request().Body)        // 10
	defer ctx.Request().Body.Close()

	b.uploads[uuid] = buf.Bytes()
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", namespace, uuid)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", buf.Len()-1))
	return ctx.NoContent(http.StatusAccepted)
}
