package skynet

import (
	"bytes"
	"encoding/json"
	"io"

	skynet "github.com/NebulousLabs/go-skynet/v2"
)

type (
	Client struct {
		skynet     *skynet.SkynetClient
		isRemote   bool
		host       string
		gatewayURL string
	}
	Config struct {
		Host       string
		GatewayURL string
	}

	SkynetMeta struct {
		SkyLink string
		Name    string
		Size    uint64
		Type    int
	}

	Image struct {
		Layers map[string][]byte
		Manifests map[string][]byte
	}
)

func (i Image) Reader() (io.Reader, error) {
	bz, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(bz), nil
}
