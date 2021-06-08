package registry

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

// type RegistryErrors struct {
// 	Errors []RegistryError `json:"errors"`
// }

// type RegistryError struct {
// 	Code    string                 `json:"code"`
// 	Message string                 `json:"message"`
// 	Detail  map[string]interface{} `json:"detail,omitempty"`
// }

// // OCI - Distribution Spec compliant Headers
// const (
// 	HeaderDockerContentDigest          = "Docker-Content-Digest"
// 	HeaderDockerDistributionApiVersion = "Docker-Distribution-API-Version"
// )

// // OCI - Distribution Spec compliant Error Codes
// const (
// 	RegistryErrorCodeUnknown             = "UNKNOWN"               // error unknown to registry
// 	RegistryErrorCodeBlobUnknown         = "BLOB_UNKNOWN"          //blob unknown to registry
// 	RegistryErrorCodeBlobUploadInvalid   = "BLOB_UPLOAD_INVALID"   //blob upload invalid
// 	RegistryErrorCodeBlobUploadUnknown   = "BLOB_UPLOAD_UNKNOWN"   // blob upload unknown to registry
// 	RegistryErrorCodeDigestInvalid       = "DIGEST_INVALID"        // provided digest did not match uploaded content
// 	RegistryErrorCodeManifestBlobUnknown = "MANIFEST_BLOB_UNKNOWN" // blob unknown to registry
// 	RegistryErrorCodeManifestInvalid     = "MANIFEST_INVALID"      // manifest invalid
// 	RegistryErrorCodeManifestUnknown     = "MANIFEST_UNKNOWN"      // manifest unknown
// 	RegistryErrorCodeManifestUnverified  = "MANIFEST_UNVERIFIED"   // manifest failed sign verification
// 	RegistryErrorCodeNameInvalid         = "NAME_INVALID"          // invalid repository name
// 	RegistryErrorCodeNameUnknown         = "NAME_UNKNOWN"          // repository name not known to registry
// 	RegistryErrorCodeSizeInvalid         = "SIZE_INVALID"          //provided length did not match content length
// 	RegistryErrorCodeTagInvalid          = "TAG_INVALID"           // manifest tag did not match URI
// 	RegistryErrorCodeUnauthorized        = "UNAUTHORIZED"          // authentication is required
// 	RegistryErrorCodeDenied              = "DENIED"                // request access to resource is denied
// 	RegistryErrorCodeUnsupported         = "UNSUPPORTED"           // operation is not supported
// )
