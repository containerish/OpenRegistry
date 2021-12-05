package skynet

import (
	"bytes"
	"io"
	"log"
	"strconv"

	"github.com/SkynetLabs/go-skynet/v2"
	"github.com/containerish/OpenRegistry/config"
	tar "github.com/whyrusleeping/tar-utils"
)

func NewClient(c *config.RegistryConfig) *Client {

	opts := skynet.Options{
		CustomUserAgent: c.SkynetConfig.CustomUserAgent,
	}

	skynetClient := skynet.NewCustom(c.SkynetPortalURL, opts)
	//skynetClient := skynet.New()

	return &Client{
		skynet:     &skynetClient,
		isRemote:   false,
		host:       c.Host,
		gatewayURL: c.SkynetPortalURL,
	}
}

func (c *Client) Upload(namespace, digest string, content []byte, headers ...skynet.Header) (string, error) {
	opts := skynet.DefaultUploadOptions

	opts.CustomDirname = namespace

	data := make(skynet.UploadData)

	buf := bytes.NewBuffer(content)

	data[digest] = buf

	return c.skynet.Upload(data, opts, headers...)
}

func (c *Client) Download(path string, headers ...skynet.Header) (io.ReadCloser, error) {
	opts := skynet.DefaultDownloadOptions

	return c.skynet.Download(path, opts, headers...)
}

func (c *Client) DownloadDir(skynetLink, dir string, headers ...skynet.Header) error {
	opts := skynet.DefaultDownloadOptions

	tarball, err := c.skynet.Download(skynetLink, opts, headers...)
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
func (c *Client) AddImage(ns string, mf, l map[string][]byte, headers ...skynet.Header) (string, error) {
	opts := skynet.DefaultUploadOptions
	opts.CustomDirname = ns

	uploadData := make(skynet.UploadData)

	imageReader, err := Image{mf, l}.Reader()
	if err != nil {
		return "", err
	}

	uploadData["image"] = imageReader

	link, err := c.skynet.Upload(uploadData, opts, headers...)
	return link, err
}

func (c *Client) Metadata(skylink string) (uint64, bool) {
	opts := skynet.DefaultMetadataOptions
	info, err := c.skynet.Metadata(skylink, opts)
	if err != nil {
		log.Printf("error getting metadat: %s", err)
		return 0, false
	}

	ct := info.Get("content-length")
	if ct == "" {
		return 0, false
	}

	contentLength, err := strconv.ParseUint(ct, 10, 64)
	if err != nil {
		return 0, false
	}

	return contentLength, true
}
