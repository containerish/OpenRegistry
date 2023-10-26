package registry

import (
	"sync"
	"time"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	dfsImpl "github.com/containerish/OpenRegistry/dfs"
	store_v2 "github.com/containerish/OpenRegistry/store/v2/registry"
	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
	"github.com/uptrace/bun"
)

/*
URI Specification:

Client Flow: /v2/<name>/

For authorized images -> /v2/library/ubuntu/

Rules:
1. repo name is a collection of path components.

2. each component MUST be atleast one lowercase, alpha numeric chars,
optionally separated by periods(.), dashes(-) or underscores(_) and must match the following regex:
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
	Detail  map[string]interface{} `json:"detail,omitempty"`
	Code    string                 `json:"code"`
	Message string                 `json:"message,omitempty"`
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
	RegistryErrorCodeReferrerUnknown     = "REFERRER_UNKOWN"
)

type (
	registry struct {
		b      blobs
		config *config.OpenRegistryConfig
		logger telemetry.Logger
		store  store_v2.RegistryStore
		dfs    dfsImpl.DFS
		txnMap map[string]TxnStore
		mu     *sync.RWMutex
		debug  bool
	}

	TxnStore struct {
		txn         *bun.Tx
		blobDigests []string
		timeout     time.Duration
	}

	blobs struct {
		mu                 *sync.RWMutex
		contents           map[string][]byte
		uploads            map[string][]byte
		layers             map[string][]string
		registry           *registry
		blobCounter        map[string]int64
		layerLengthCounter map[string]int64
		layerParts         map[string][]s3types.CompletedPart
	}

	ManifestList struct {
		MediaType string `json:"mediaType"`
		Manifests []struct {
			MediaType string `json:"mediaType"`
			Digest    string `json:"digest"`
			Platform  struct {
				Architecture string   `json:"architecture"`
				Os           string   `json:"os"`
				Features     []string `json:"features"`
			} `json:"platform"`
			Size int `json:"size"`
		} `json:"manifests"`
		SchemaVersion int `json:"schemaVersion"`
	}

	ImageManifest struct {
		Subject       *types.ReferrerManifest `json:"subject"`
		MediaType     string                  `json:"mediaType"`
		Config        Config                  `json:"config"`
		Layers        Layers                  `json:"layers"`
		SchemaVersion int                     `json:"schemaVersion"`
	}

	Layers []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	}

	Config struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
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
	DeleteTagOrManifest(ctx echo.Context) error
	//The list of available repositories is made available through the catalog
	Catalog(ctx echo.Context) error
	GetImageNamespace(ctx echo.Context) error

	// MonolithicPut is used as the second operation for MonolithicUpload with POST + Put
	MonolithicPut(ctx echo.Context) error

	// Create Repository
	CreateRepository(ctx echo.Context) error

	GetReferrers(ctx echo.Context) error
}
