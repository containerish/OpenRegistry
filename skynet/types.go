package skynet

import (
	"bytes"
	"encoding/json"
	"io"

	skynet "github.com/SkynetLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/config"
)

type (
	Client struct {
		skynet     *skynet.SkynetClient
		config     *config.OpenRegistryConfig
		host       string
		gatewayURL string
		isRemote   bool
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
		Layers    map[string][]byte
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
