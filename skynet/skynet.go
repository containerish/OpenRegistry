package skynet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SkynetLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/config"
	tar "github.com/whyrusleeping/tar-utils"
)

func NewClient(c *config.RegistryConfig) *Client {

	opts := skynet.Options{
		CustomUserAgent: c.SkynetConfig.CustomUserAgent,
		APIKey:          c.SkynetConfig.ApiKey,
		CustomCookie:    c.SkynetConfig.ApiKey,
	}

	skynetClient := skynet.NewCustom(c.SkynetPortalURL, opts)
	httpClient := http.DefaultClient
	httpClient.Timeout = time.Second * 60

	return &Client{
		skynet:     &skynetClient,
		httpClient: httpClient,
		isRemote:   false,
		host:       c.Host,
		gatewayURL: c.SkynetPortalURL,
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

	if pin {
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

func (c *Client) Metadata(skylink string) (uint64, bool) {
	skl := strings.TrimPrefix(skylink, "sia://")
	url := fmt.Sprintf("%s/%s", c.skynet.PortalURL, skl)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, url, nil)
	if err != nil {
		return 0, false
	}

	// req.SetBasicAuth("", c.skynet.Options.APIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Length")
	if ct == "" {
		return 0, false
	}

	contentLength, err := strconv.ParseUint(ct, 10, 64)
	if err != nil {
		return 0, false
	}

	return contentLength, true
}
