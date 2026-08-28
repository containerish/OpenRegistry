package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
)

func (re RegistryErrors) Error() string {
	if len(re.Errors) == 0 {
		panic("error is nil")
	}

	bz, _ := json.Marshal(re)
	return string(bz)
}

func (re *RegistryError) Error() string {
	if re == nil {
		panic("error is nil")
	}

	bz, _ := json.Marshal(re)
	return string(bz)
}

func (re RegistryErrors) Bytes() []byte {
	bz, err := json.Marshal(re)
	if err != nil {
		color.Red("error marshaling error response: %w", err)
	}

	return bz
}

type (
	RegistryErrors struct {
		Errors []RegistryError `json:"errors"`
	}

	RegistryError struct {
		Detail  map[string]interface{} `json:"detail,omitempty"`
		Code    string                 `json:"code"`
		Message string                 `json:"message,omitempty"`
	}

	ObjectMetadata struct {
		ContentType   string
		Etag          string
		DFSLink       string
		ContentLength int
	}

	Metadata struct {
		Namespace string
		Manifest  ImageManifest
	}

	ImageManifest struct {
		MediaType     string    `json:"mediaType"`
		Layers        []*Layer  `json:"layers"`
		Config        []*Config `json:"config"`
		SchemaVersion int       `json:"schemaVersion"`
	}

	ImageManifestV2 struct {
		CreatedAt     time.Time `json:"created_at,omitempty"`
		UpdatedAt     time.Time `json:"updated_at,omitempty"`
		Uuid          string    `json:"uuid,omitempty"`
		Namespace     string    `json:"namespace"`
		MediaType     string    `json:"mediaType,omitempty"`
		SchemaVersion int       `json:"schemaVersion,omitempty"`
	}

	Blob struct {
		CreatedAt  time.Time
		Digest     string
		DFSLink    string
		UUID       string
		RangeStart uint32
		RangeEnd   uint32
	}

	Layer struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		DFSLink   string `json:"dfsLink"`
		UUID      string `json:"uuid"`
		Blobs     []Blob `json:"blobs"`
		Size      int    `json:"size"`
	}

	LayerV2 struct {
		CreatedAt   time.Time `json:"created_at,omitempty"`
		UpdatedAt   time.Time `json:"updated_at,omitempty"`
		MediaType   string    `json:"mediaType"`
		Digest      string    `json:"digest"`
		DFSLink     string    `json:"dfsLink"`
		UUID        string    `json:"uuid"`
		BlobDigests []string  `json:"blobs"`
		Size        int       `json:"size"`
	}

	LayerRef struct {
		Digest  string
		DFSLink string
	}

	Config struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		DFSLink   string `json:"dfsLink"`
		Reference string `json:"reference"`
		Size      int    `json:"size"`
	}

	ConfigV2 struct {
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		UUID      string    `json:"uuid,omitempty"`
		Namespace string    `json:"namespace,omitempty"`
		DFSLink   string    `json:"dfs_link,omitempty"`
		MediaType string    `json:"media_type,omitempty"`
		Reference string    `json:"reference"`
		Digest    string    `json:"digest"`
		Layers    []string  `json:"layers,omitempty"`
		Size      int       `json:"size,omitempty"`
	}

	Catalog struct {
		Repositories []*Repository `json:"repositories"`
	}

	Repository struct {
		Namespace string      `json:"namespace"`
		Tags      []*ConfigV2 `json:"tags"`
	}
)

func (md Metadata) GetManifestByRef(ref string) (*Config, error) {
	if len(md.Manifest.Config) == 0 {
		return nil, fmt.Errorf("manifest not found")
	}

	for _, c := range md.Manifest.Config {
		if c.Digest == ref || c.Reference == ref {
			return c, nil
		}
	}

	return nil, fmt.Errorf("manifest not found")
}

func (md Metadata) Digests() []string {
	digests := make([]string, len(md.Manifest.Config))

	for _, c := range md.Manifest.Config {
		digests = append(digests, c.Digest)
	}

	for _, l := range md.Manifest.Layers {
		digests = append(digests, l.Digest)
	}

	return digests
}

func (md Metadata) Bytes() []byte {
	bz, err := json.Marshal(md)
	if err != nil {
		fmt.Println(err.Error())
		return []byte{}
	}

	return bz
}

func (md Metadata) FindLayer(ref string) *Layer {
	for _, l := range md.Manifest.Layers {
		if l.Digest == ref {
			return l
		}
	}

	return nil
}

func (md Metadata) FindLinkForDigest(ref string) (string, error) {
	for _, c := range md.Manifest.Config {
		if c.Digest == ref || c.Reference == ref {
			return c.DFSLink, nil
		}
	}

	for _, l := range md.Manifest.Layers {
		if l.Digest == ref {
			return l.DFSLink, nil
		}
	}

	return "", fmt.Errorf("ref does not exists")
}

func (lr LayerRef) Bytes() []byte {
	bz, err := json.Marshal(lr)
	if err != nil {
		return nil
	}

	return bz
}
