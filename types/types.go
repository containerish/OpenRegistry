package types

import (
	"encoding/json"
	"fmt"
)

type (
	Metadata struct {
		Namespace string
		Manifest  ImageManifest
	}

	Manifest struct {
		SkynetLink string
		Reference  string
		Digest     string
	}

	ImageManifest struct {
		MediaType     string    `json:"mediaType"`
		Layers        []*Layer  `json:"layers"`
		Config        []*Config `json:"config"`
		SchemaVersion int       `json:"schemaVersion"`
	}

	Blob struct {
		Digest     string
		Skylink    string
		UUID       string
		RangeStart uint32
		RangeEnd   uint32
	}

	Layer struct {
		MediaType  string `json:"mediaType"`
		Digest     string `json:"digest"`
		SkynetLink string `json:"skynetLink"`
		UUID       string `json:"uuid"`
		Blobs      []Blob `json:"blobs"`
		Size       int    `json:"size"`
	}

	LayerRef struct {
		Digest  string
		Skylink string
	}

	Config struct {
		MediaType  string `json:"mediaType"`
		Digest     string `json:"digest"`
		SkynetLink string `json:"skynetLink"`
		Reference  string `json:"reference"`
		Size       int    `json:"size"`
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
			return c.SkynetLink, nil
		}
	}

	for _, l := range md.Manifest.Layers {
		if l.Digest == ref {
			return l.SkynetLink, nil
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
