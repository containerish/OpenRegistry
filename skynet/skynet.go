package skynet

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SkynetLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/config"
	"github.com/fatih/color"
	tar "github.com/whyrusleeping/tar-utils"
)

func NewClient(oc *config.OpenRegistryConfig) *Client {

	opts := skynet.Options{
		CustomUserAgent: oc.SkynetConfig.CustomUserAgent,
		SkynetAPIKey:    oc.SkynetConfig.ApiKey,
		HttpClient:      newHttpClientForSkynet(),
	}

	color.Green("Skynet Portal: %s", oc.SkynetConfig.SkynetPortalURL)
	skynetClient := skynet.NewCustom(oc.SkynetConfig.SkynetPortalURL, opts)

	return &Client{
		skynet:     &skynetClient,
		isRemote:   false,
		host:       oc.Registry.Host,
		gatewayURL: oc.SkynetConfig.SkynetPortalURL,
		config:     oc,
	}
}

func (c *Client) Upload(namespace, digest string, content []byte, pin bool) (string, error) {
	opts := skynet.DefaultUploadOptions
	opts.SkynetAPIKey = c.skynet.Options.SkynetAPIKey
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
	retryCounter := 3

	var err error
	var metadata *skynet.Metadata
	for i := retryCounter; retryCounter != 0; i-- {
		metadata, err = c.skynet.Metadata(skylink, skynet.DefaultMetadataOptions)
		if err != nil {
			err = fmt.Errorf("SKYNET_METADATA_ERR: %w", err)
			retryCounter--
			// cool off
			time.Sleep(time.Second * 3)
			continue
		}
		break
	}

	return metadata, err
}

func newHttpClientForSkynet() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	return &http.Client{
		Transport:     t,
		CheckRedirect: http.DefaultClient.CheckRedirect,
		Timeout:       time.Minute * 20, //sounds super risky maybe find an alternative
	}
}
