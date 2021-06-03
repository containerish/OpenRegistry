package skynet

import (
	"io"

	"github.com/NebulousLabs/go-skynet/v2"
	"github.com/fatih/color"
	"github.com/jay-dee7/parachute/config"
	tar "github.com/whyrusleeping/tar-utils"
)

func NewClient(c *config.RegistryConfig) *Client {

	opts := skynet.Options{
		EndpointPath:    "",
		APIKey:          "",
		CustomUserAgent: "",
	}

	skynet.NewCustom(c.SkynetPortalURL, opts)
	skynetClient := skynet.New()

	return &Client{
		skynet:     &skynetClient,
		isRemote:   false,
		host:       c.Host,
		gatewayURL: c.SkynetPortalURL,
	}
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

func (c *Client) UploadDirectory(dirPath string) (string, error) {
	opts := skynet.DefaultUploadOptions
	link, err := c.skynet.UploadDirectory(dirPath, opts)
	color.Red(link)

	return link, err
}

func (c *Client) List(path string) ([]*SkynetMeta, error) {
	return nil, nil
}

func (c *Client) AddImage(manifests map[string][]byte, layers map[string][]byte) (string, error) {
	opts := skynet.DefaultUploadOptions

	uploadData := make(skynet.UploadData)

	imageReader, err := Image{manifests, layers}.Reader()
	if err != nil {
		return "", err
	}

	uploadData["image"] = imageReader

	link, err := c.skynet.Upload(uploadData, opts)
	color.Red(link)
	return link, err
}
