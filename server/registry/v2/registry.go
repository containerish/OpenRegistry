package registry

import (
	"sync"

	"github.com/jay-dee7/parachute/cache"
	"github.com/jay-dee7/parachute/skynet"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

/*
URI Specification:

Client Flow: /v2/<name>/

For authorized images -> /v2/library/ubuntu/

Rules:
1. repo name is a collection of path components.

2. each component MUST be atleast one lowercase, alpha numeric chars, optionally separated by periods(.), dashes(-) or underscores(_) and must match the following regex:
[a-z0-9]+(?:[._-][a-z0-9]+)*

3. if repo name has two or more path components, they must be separated by forward slashes (/)

4. total length (including slashes) must be less than 256 chars

/*


/*

Error Format

{
    "errors:" [{
            "code": <error identifier>,
            "message": <message describing condition>,
            "detail": <unstructured>
        },
        ...
    ]
}

*/

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

// // OCI - Distribution Spec compliant Error Codes
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

type (
	registry struct {
		log        zerolog.Logger
		debug      bool
		skynet     *skynet.Client
		b          blobs
		localCache cache.Store
		echoLogger echo.Logger
		mu         *sync.RWMutex
	}

	blobs struct {
		mutex    sync.Mutex
		contents map[string][]byte
		uploads  map[string][]byte
		layers   map[string][]string
		registry *registry
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
		Layers        Layers `json:"layers"`
		Config        Config `json:"config"`
	}

	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	}

	Config struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	}
)

type Registry interface {
	UploadProgress(ctx echo.Context) error

	// GET /v2/<name>/blobs/<digest>
	PullLayer(ctx echo.Context) error

	// GET /v2/
	ApiVersion(ctx echo.Context) error

	// HEAD /v2/<name>/manifests/<ref>
	ManifestExists(ctx echo.Context) error

	// HEAD /v2/<name>/blobs/<digest>
	LayerExists(ctx echo.Context) error

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

	StartUpload(ctx echo.Context) error

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

	Length(ctx echo.Context) error
}
