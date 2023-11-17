package types

type (
	Metadata struct {
		Namespace string
		Manifest  ImageManifest
	}

	ObjectMetadata struct {
		ContentType   string
		Etag          string
		DFSLink       string
		ContentLength int
	}
)
