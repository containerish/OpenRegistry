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
		SchemaVersion int     `json:"schemaVersion"`
		MediaType     string  `json:"mediaType"`
		Layers        []*Layer `json:"layers"`
		Config        Config  `json:"config"`
	}

	Layer struct {
		MediaType  string `json:"mediaType"`
		Size       int    `json:"size"`
		Digest     string `json:"digest"`
		SkynetLink string `json:"skynetLink"`
		UUID       string `json:"uuid"`
	}

	Config struct {
		MediaType  string `json:"mediaType"`
		Size       int    `json:"size"`
		Digest     string `json:"digest"`
		SkynetLink string `json:"skynetLink"`
		Reference  string `json:"reference"`
	}
)

func (md Metadata) Digests() []string{
	digests := []string{md.Manifest.Config.Digest}

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

func (md Metadata) FindLinkForDigest(ref string) (string, error) {
	for _, l := range md.Manifest.Layers {
		if l.Digest == ref {
			return l.SkynetLink, nil
		}
	}

	if md.Manifest.Config.Digest == ref {
		return md.Manifest.Config.SkynetLink, nil
	}

	return "", fmt.Errorf("ref does not exists")
}
