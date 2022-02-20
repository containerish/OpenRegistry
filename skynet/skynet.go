package skynet

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SkynetLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/config"
	tar "github.com/whyrusleeping/tar-utils"
)

func NewClient(oc *config.OpenRegistryConfig) *Client {

	opts := skynet.Options{
		CustomUserAgent: oc.SkynetConfig.CustomUserAgent,
		APIKey:          oc.SkynetConfig.ApiKey,
		CustomCookie:    oc.SkynetConfig.ApiKey,
	}

	skynetClient := skynet.NewCustom(oc.SkynetConfig.SkynetPortalURL, opts)
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Second * 60

	return &Client{
		skynet:     &skynetClient,
		httpClient: httpClient,
		isRemote:   false,
		host:       oc.Registry.Host,
		gatewayURL: oc.SkynetConfig.SkynetPortalURL,
		config:     oc,
	}
}

func (c *Client) Upload(namespace, digest string, content []byte, pin bool) (string, error) {
	opts := skynet.DefaultUploadOptions
	opts.APIKey = c.skynet.Options.APIKey
	opts.CustomDirname = namespace

	data := make(skynet.UploadData)
	buf := bytes.NewBuffer(content)

	data[digest] = buf

	skylink, err := c.skynet.Upload(data, opts)
	if err != nil {
		return "", err
	}

	// enable pinning only in Prod Environment
	if pin && c.config.Environment == config.Prod {
		return c.skynet.PinSkylink(skylink)
	}

	return skylink, nil
}

func (c *Client) Download(path string) (io.ReadCloser, error) {
	opts := skynet.DefaultDownloadOptions

	return c.skynet.Download(path, opts)
}

func (c *Client) DownloadDir(skynetLink, dir string) error {
	opts := skynet.DefaultDownloadOptions

	tarball, err := c.skynet.Download(skynetLink, opts)
	if err != nil {
		return err
	}
	defer tarball.Close()

	ext := &tar.Extractor{Path: dir}

	return ext.Extract(tarball)
}

func (c *Client) List(path string) ([]*SkynetMeta, error) {
	return nil, nil
}

// AddImage - arguments: ns = namespace, mf = manifest and l = layers
func (c *Client) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	opts := skynet.DefaultUploadOptions
	opts.CustomDirname = ns

	uploadData := make(skynet.UploadData)

	imageReader, err := Image{mf, l}.Reader()
	if err != nil {
		return "", err
	}

	uploadData["image"] = imageReader

	link, err := c.skynet.Upload(uploadData, opts)
	return link, err
}

func (c *Client) Metadata(skylink string) (*skynet.Metadata, error) {
	metadata, err := c.skynet.Metadata(skylink, skynet.DefaultMetadataOptions)
	if err != nil {
		return nil, fmt.Errorf("SKYNET_METADATA_ERR: %w", err)
	}

	return metadata, nil
}
