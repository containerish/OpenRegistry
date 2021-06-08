package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	skynetsdk "github.com/NebulousLabs/go-skynet/v2"
	"github.com/docker/distribution/uuid"
	"github.com/fatih/color"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/jay-dee7/parachute/cache"
	"github.com/jay-dee7/parachute/skynet"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type (
	registry struct {
		log        zerolog.Logger
		debug      bool
		skynet     *skynet.Client
		b          blobs
		localCache cache.Store
	}

	blobs struct {
		mutex    sync.Mutex
		contents map[string][]byte
		uploads  map[string][]byte
		layers   map[string][]string
	}

	logMsg map[string]interface{}

	ManifestList struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Manifests     []struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
			Platform  struct {
				Architecture string   `json:"architecture"`
				Os           string   `json:"os"`
				Features     []string `json:"features"`
			} `json:"platform"`
		} `json:"manifests"`
	}

	ImageManifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		Config        struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		} `json:"layers"`
	}
)

type RegistryErrors struct {
	Errors []RegistryError `json:"errors"`
}

type RegistryError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
}

// OCI - Distribution Spec compliant Headers
const (
	HeaderDockerContentDigest          = "Docker-Content-Digest"
	HeaderDockerDistributionApiVersion = "Docker-Distribution-API-Version"
)

// OCI - Distribution Spec compliant Error Codes
const (
	RegistryErrorCodeUnknown             = "UNKNOWN"               // error unknown to registry
	RegistryErrorCodeBlobUnknown         = "BLOB_UNKNOWN"          //blob unknown to registry
	RegistryErrorCodeBlobUploadInvalid   = "BLOB_UPLOAD_INVALID"   //blob upload invalid
	RegistryErrorCodeBlobUploadUnknown   = "BLOB_UPLOAD_UNKNOWN"   // blob upload unknown to registry
	RegistryErrorCodeDigestInvalid       = "DIGEST_INVALID"        // provided digest did not match uploaded content
	RegistryErrorCodeManifestBlobUnknown = "MANIFEST_BLOB_UNKNOWN" // blob unknown to registry
	RegistryErrorCodeManifestInvalid     = "MANIFEST_INVALID"      // manifest invalid
	RegistryErrorCodeManifestUnknown     = "MANIFEST_UNKNOWN"      // manifest unknown
	RegistryErrorCodeManifestUnverified  = "MANIFEST_UNVERIFIED"   // manifest failed sign verification
	RegistryErrorCodeNameInvalid         = "NAME_INVALID"          // invalid repository name
	RegistryErrorCodeNameUnknown         = "NAME_UNKNOWN"          // repository name not known to registry
	RegistryErrorCodeSizeInvalid         = "SIZE_INVALID"          //provided length did not match content length
	RegistryErrorCodeTagInvalid          = "TAG_INVALID"           // manifest tag did not match URI
	RegistryErrorCodeUnauthorized        = "UNAUTHORIZED"          // authentication is required
	RegistryErrorCodeDenied              = "DENIED"                // request access to resource is denied
	RegistryErrorCodeUnsupported         = "UNSUPPORTED"           // operation is not supported
)

func NewRegistry(logger zerolog.Logger, c cache.Store) (Registry, error) {
	return &registry{log: logger, localCache: c}, nil
}

func (r *registry) errorResponse(code, msg string, detail map[string]interface{}) []byte {
	var err RegistryErrors

	err.Errors = append(err.Errors, RegistryError{
		Code:    code,
		Message: msg,
		Detail:  detail,
	})

	bz, e := json.Marshal(err)
	if e != nil {
		lm := make(logMsg)
		lm["error"] = e.Error()
		r.debugf(lm)
	}

	return bz
}

func (r *registry) DeleteLayer(ctx echo.Context) error {
	return nil
}

func (r *registry) MonolithicUpload(ctx echo.Context) error {
	return ctx.NoContent(http.StatusNotImplemented)
}

// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
func (r *registry) CompleteUpload(ctx echo.Context) error {
	digest := ctx.QueryParam("digest")
	namespace := ctx.Param("namespace")
	uuid := ctx.Param("uuid")
	contentRange := ctx.Request().Header.Get("Content-Range")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	hash := sha256.Sum256(bz)
	dockerfy := "sha256:" + hex.EncodeToString(hash[:])

	if dockerfy != digest {
		details := map[string]interface{}{
			"headerDigest": digest, "serverSideDigest": dockerfy,
		}
		errMsg := r.errorResponse(RegistryErrorCodeDigestInvalid, "digest mismatch", details)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	var headers []skynetsdk.Header

	for k, v := range ctx.Request().Header {
		headers = append(headers, skynetsdk.Header{
			Key: k, Value: v[0],
		})
	}

	skylink, err := r.skynet.Upload(digest, ctx.Request().Body, headers...)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	key := namespace + "/" + uuid

	val := map[string]string{
		"namespace":    namespace,
		"uuid":         uuid,
		"contentRange": contentRange,
		"skynetUrl":    skylink,
	}

	bz, err = json.Marshal(val)
	if err != nil {
		color.Red("error marshaling JSON: %s\n", err.Error())
	}

	r.localCache.Update([]byte(key), bz)

	locationHeader := fmt.Sprintf("/v2/%s/blobs/%s", namespace, digest)
	ctx.Response().Header().Set("Content-Length", "0")
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	ctx.Response().Header().Set("Location", locationHeader)

	return ctx.NoContent(http.StatusCreated)
}

func (r *registry) getDigestFromURI(u *url.URL) (string, *RegistryError) {

	elem := strings.Split(u.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		return "", &RegistryError{
			Code:    RegistryErrorCodeNameInvalid,
			Message: "blobs must be attached to a repo",
			Detail:  map[string]interface{}{},
		}
	}

	return elem[len(elem)-1], nil
}

// HEAD /v2/<name>/blobs/<digest>
// 200 OK
// Content-Length: <length of blob>
// Docker-Content-Digest: <digest>
func (r *registry) LayerExists(ctx echo.Context) error {
	return r.ManifestExists(ctx)
}

// HEAD /v2/<name>/manifests/<reference>
func (r *registry) ManifestExists(ctx echo.Context) error {
	namespace := ctx.Get("namespace").(string)
	ref := ctx.Get("reference").(string) // ref can be either tag or digest
	digest, rerr := r.getDigestFromURI(ctx.Request().URL)
	if rerr != nil {
		return ctx.String(http.StatusNotFound, "Not Found")
	}

	key, err := r.localCache.GetSkynetURL(namespace, ref)
	if err != nil {
		details := echo.Map{
			"skynet": "skynet link not found",
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	bz, err := r.localCache.Get([]byte(key))
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	var meta skynet.SkynetMeta
	if err = json.Unmarshal(bz, &meta); err != nil {
		details := echo.Map{
			"error": err.Error(),
		}
		errMsg := r.errorResponse(RegistryErrorCodeManifestBlobUnknown, err.Error(), details)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	return ctx.String(http.StatusOK, "OK")
}

// PATCH /v2/<name>/blobs/uploads/<uuid>
func (r *registry) ChunkedUpload(ctx echo.Context) error {
	namespace := ctx.Get("namespace").(string)
	uuid := ctx.Get("uuid").(string)
	contentRange := ctx.Get("Content-Range").(string)

	elem := strings.Split(ctx.Request().URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		errMsg := r.errorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	digest, rerr := r.getDigestFromURI(ctx.Request().URL)
	if rerr != nil {
		return ctx.String(http.StatusNotFound, "Not Found")
	}

	start, end := 0, 0
	if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	// b.lock.Lock()
	// defer b.lock.Unlock()

	if start != len(r.b.uploads[digest]) {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadUnknown, "content mismatch", nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	// r.b.uploads[digest] = l.Bytes()
	var headers []skynetsdk.Header

	for k, v := range ctx.Request().Header {
		headers = append(headers, skynetsdk.Header{
			Key: k, Value: v[0],
		})
	}

	skylink, err := r.skynet.Upload(digest, ctx.Request().Body, headers...)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUploadInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusRequestedRangeNotSatisfiable, errMsg)
	}

	key := namespace + "/" + uuid

	val := map[string]string{
		"namespace":    namespace,
		"uuid":         uuid,
		"contentRange": contentRange,
		"skynetUrl":    skylink,
	}

	bz, err := json.Marshal(val)
	if err != nil {
		color.Red("error marshaling JSON: %s\n", err.Error())
	}

	r.localCache.Update([]byte(key), bz)

	p := path.Join(elem[1 : len(elem)-3]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, uuid)

	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Range", fmt.Sprintf("0-%d", ctx.Request().ContentLength-1))
	ctx.Response().WriteHeader(http.StatusNoContent)
	return nil
}

func (r *registry) CancelUpload(ctx echo.Context) error {
	return nil
}

// GET /v2/<name>/manifests/<reference>
func (r *registry) PullManifest(ctx echo.Context) error {
	namespace := ctx.Get("namespace").(string)
	ref := ctx.Get("reference").(string)

	skynetLink, err := r.localCache.GetSkynetURL(namespace, ref)
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

	bz, err := io.ReadAll(resp)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSON(http.StatusNotFound, errMsg)
	}

	return ctx.JSONBlob(http.StatusOK, bz)
}

func (r *registry) digest(bz []byte) string {
	hash := sha256.Sum256(bz)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (r *registry) PushManifest(ctx echo.Context) error {
	namespace := ctx.Param("namespace")

	bz, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeManifestInvalid, err.Error(), nil)
		return ctx.JSONBlob(http.StatusBadRequest, errMsg)
	}

	digest := r.digest(bz)
	mf := ImageManifest{
		SchemaVersion: 2,
		MediaType:     ctx.Request().Header.Get("Content-Type"),
		Config: struct {
			MediaType string "json:\"mediaType\""
			Size      int    "json:\"size\""
			Digest    string "json:\"digest\""
		}{},
		Layers: []struct {
			MediaType string "json:\"mediaType\""
			Size      int    "json:\"size\""
			Digest    string "json:\"digest\""
		}{},
	}

	if mf.MediaType == string(types.OCIImageIndex) || mf.MediaType == string(types.DockerManifestList) {
		var mfList ManifestList
		if err := json.NewDecoder(ctx.Request().Body).Decode(&mfList); err != nil {
			errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, err.Error(), nil)
			return ctx.JSONBlob(http.StatusNotFound, errMsg)
		}

		for _, desc := range mfList.Manifests {
			val, _ := r.localCache.Get([]byte(namespace))
			var m map[string]string
			_ = json.Unmarshal(val, &m)
			if _, found := m[desc.Digest]; !found {
				errMsg := r.errorResponse(RegistryErrorCodeManifestUnknown, "sub-manifest not found: "+desc.Digest, nil)
				return ctx.JSONBlob(http.StatusNotFound, errMsg)
			}
		}
	}

	r.skynet.AddImage(manifests, layers)

	return nil
}

func (r *registry) DeleteImage(ctx echo.Context) error {
	return nil
}

// GET /v2/<name>/blobs/<digest>
func (r *registry) PullLayer(ctx echo.Context) error {
	namespace := ctx.Get("namespace").(string)
	digest, rerr := r.getDigestFromURI(ctx.Request().URL)
	if rerr != nil {
		bz, _ := json.Marshal(rerr)
		return ctx.JSONBlob(http.StatusNotFound, bz)
	}

	skynetlink, err := r.localCache.GetSkynetURL(namespace, digest)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	resp, err := http.DefaultClient.Get(skynetlink)
	if err != nil {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errMsg := r.errorResponse(RegistryErrorCodeBlobUnknown, err.Error(), nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}

	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	ctx.Response().Header().Set("Docker-Content-Digest", digest)
	return ctx.Stream(http.StatusOK, "application/octet-stream", resp.Body)
}

func (r *registry) BlobMount(ctx echo.Context) error {
	return nil
}
func (r *registry) PushImage(ctx echo.Context) error {
	return nil
}

func (r *registry) StartUpload(ctx echo.Context) error {
	elem := strings.Split(ctx.Request().URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		// return &restError{
		// 	Status:  http.StatusBadRequest,
		// 	Code:    "NAME_INVALID",
		// 	Message: "blobs must be atjjtached to a repo",
		// }
		errMsg := r.errorResponse(RegistryErrorCodeNameInvalid, "blobs must be attached to a repo", nil)
		return ctx.JSONBlob(http.StatusNotFound, errMsg)
	}
	id := ""
	p := path.Join(elem[1 : len(elem)-2]...)
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, id)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", id)
	ctx.Response().Header().Set("Range", "0-0")
	return ctx.NoContent(http.StatusAccepted)
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
	locationHeader := fmt.Sprintf("/v2/%s/blobs/uploads/%s", p, id)
	ctx.Response().Header().Set("Location", locationHeader)
	ctx.Response().Header().Set("Docker-Upload-UUID", id.String())
	ctx.Response().Header().Set("Range", "0-0")
	return ctx.NoContent(http.StatusAccepted)
}

func (r *registry) ListTags(ctx echo.Context) error {
	return nil
}
func (r *registry) List(ctx echo.Context) error {
	return nil
}

func (r *registry) debugf(lm logMsg) {
	if r.debug {
		e := r.log.Debug()
		e.Fields(lm).Send()
	}
}

// Should also look into 401 Code
// https://docs.docker.com/registry/spec/api/
func (r *registry) ApiVersion(ctx echo.Context) error {
	ctx.Response().Header().Set(HeaderDockerDistributionApiVersion, "registry/2.0")
	return ctx.String(http.StatusOK, "OK")
}

type Registry interface {
	// GET /v2/<name>/blobs/<digest>
	PullLayer(ctx echo.Context) error

	// GET /v2/
	ApiVersion(ctx echo.Context) error

	// HEAD /v2/<name>/manifests/<ref>
	ManifestExists(ctx echo.Context) error

	// GET /v2/<name>/manifests/<ref>
	PullManifest(ctx echo.Context) error

	// PUT /v2/<name>/manifests/<reference>
	PushManifest(ctx echo.Context) error

	// Push individual layers first, then upload a signed manifest
	// POST /v2/<name>/blobs/uploads/
	// For existing layers:
	// make a HEAD request first like:
	// HEAD /v2/<name>/blobs/<digest>
	// Ok Response:
	// 200 OK
	// Content-Length: <length of blob>
	// Docker-Content-Digest: <digest>
	// Uploading layer:
	// 202 Accepted
	// Location: /v2/<name>/blobs/uploads/<uuid>
	// Range: bytes=0-<offset>
	// Content-Length: 0
	// Docker-Upload-UUID: <uuid>
	PushImage(ctx echo.Context) error

	PushLayer(ctx echo.Context) error

	// PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	// Content-Length: <size of layer>
	// Content-Type: application/octet-stream
	// <Layer Binary Data>
	MonolithicUpload(ctx echo.Context) error

	// PATCH /v2/<name>/blobs/uploads/<uuid>
	// Content-Length: <size of chunk>
	// Content-Range: <start of range>-<end of range>
	// Content-Type: application/octet-stream
	// <Layer Chunk Binary Data>

	// 416 Requested Range Not Satisfiable
	//Location: /v2/<name>/blobs/uploads/<uuid>
	// Range: 0-<last valid range>
	// Content-Length: 0
	// Docker-Upload-UUID: <uuid>

	/*
			202 Accepted
		    Location: /v2/<name>/blobs/uploads/<uuid>
		    Range: bytes=0-<offset>
		    Content-Length: 0
		    Docker-Upload-UUID: <uuid>
	*/
	ChunkedUpload(ctx echo.Context) error

	/*
	   PUT /v2/<name>/blobs/uploads/<uuid>?digest=<digest>
	   Content-Length: <size of chunk>
	   Content-Range: <start of range>-<end of range>
	   Content-Type: application/octet-stream
	   <Last Layer Chunk Binary Data>

	   Success Response:

	   201 Created
	   Location: /v2/<name>/blobs/<digest>
	   Content-Length: 0
	   Docker-Content-Digest: <digest>
	*/

	CompleteUpload(ctx echo.Context) error

	// DELETE /v2/<name>/blobs/uploads/<uuid>
	CancelUpload(ctx echo.Context) error

	// POST /v2/<name>/blobs/uploads/?mount=<digest>&from=<repository name>
	// Content-Length: 0

	/*
			Success Response

		201 Created
		Location: /v2/<name>/blobs/<digest>
		Content-Length: 0
		Docker-Content-Digest: <digest>
	*/
	BlobMount(ctx echo.Context) error

	// DELETE /v2/<name>/blobs/<digest>
	// 202 Accepted
	// Content-Length: None
	// 404 Not Found for not found layer
	DeleteLayer(ctx echo.Context) error

	// GET /v2/_catalog
	List(ctx echo.Context) error

	// GET /v2/<name>/tags/list
	ListTags(ctx echo.Context) error

	// DELETE /v2/<name>/manifests/<reference>
	// here ref is digest

	// Success : 202
	DeleteImage(ctx echo.Context) error
}
