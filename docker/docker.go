package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func New(c *Config) *Client {
	if c == nil {
		c = &Config{}
	}

	return newConfigFromEnv(c)
}

func newConfigFromEnv(c *Config) *Client {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("error initializing new docker client: %s\n", err)
	}

	dockerClient.NegotiateAPIVersion(ctx)

	return &Client{
		docker: dockerClient,
		debug:  c.debug,
	}
}


func (c *Client) HasImage(imageID string) (bool, error) {
	args := filters.NewArgs()
	args.Add("references", StripImageTagHost(imageID))

	images, err := c.docker.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return false, err
	}

	if len(images) == 0 {
		return true, nil
	}

	return false, nil
}

func (c *Client) ListImages() ([]*ImageSummary, error) {
	images, err := c.docker.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: filters.Args{},
	})
	if err != nil {
		return nil, err
	}
	var summaries []*ImageSummary

	for _, i := range images {
		summaries = append(summaries, &ImageSummary{
			ID:   i.ID,
			Tags: i.RepoTags,
			Size: i.Size,
		})
	}

	return summaries, nil
}

func (c *Client) PullImage(imageID string) error {
	r, err := c.docker.ImagePull(context.Background(), imageID, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	defer r.Close()

	io.Copy(io.Discard, r)
	return nil
}

func (c *Client) PushImage(imageID string) error {
	r, err := c.docker.ImagePush(context.Background(), imageID, types.ImagePushOptions{
		All:          false,
		RegistryAuth: "anything-will-work",
		PrivilegeFunc: func() (string, error) {return "", nil},
		Platform: "",
	})
	if err != nil {
		return err
	}

	if c.debug {
		io.Copy(os.Stdout, r)
	}

	return nil
}

func (c *Client) TagImage(imageID, tag string) error {
	return c.docker.ImageTag(context.Background(), imageID, tag)
}

// RemoveImage remove an image from the local registry
func (c *Client) RemoveImage(imageID string) error {
	_, err := c.docker.ImageRemove(context.Background(), imageID, types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})

	return err
}

// RemoveAllImages removes all images from the local registry
func (c *Client) RemoveAllImages() error {
	images, err := c.ListImages()
	if err != nil {
		return err
	}

	var lastErr error
	for _, image := range images {
		err := c.RemoveImage(image.ID)
		if err != nil {
			lastErr = err
			continue
		}
	}

	images, err = c.ListImages()
	if err != nil {
		return err
	}

	if len(images) != 0 {
		return lastErr
	}

	return nil
}

// ReadImage reads the contents of an image into an IO reader
func (c *Client) ReadImage(imageID string) (io.Reader, error) {
	return c.docker.ImageSave(context.Background(), []string{imageID})
}

// LoadImage loads an image from an IO reader
func (c *Client) LoadImage(input io.Reader) error {
	output, err := c.docker.ImageLoad(context.Background(), input, false)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(output.Body)
	c.Debugf("%s", string(body))

	return err
}

// LoadImageByFilePath loads an image from a tarball
func (c *Client) LoadImageByFilePath(filepath string) error {
	input, err := os.Open(filepath)
	if err != nil {
		fmt.Errorf("[docker] load image by filepath error; %v", err)
		return err
	}
	return c.LoadImage(input)
}

func (c *Client) SaveImageTar(imageID string, dest string) error {
	reader, err := c.ReadImage(imageID)
	if err != nil {
		return err
	}

	fo, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer fo.Close()

	io.Copy(fo, reader)
	return nil
}

func (c *Client) Debugf(str string, args ...interface{}) {
	if c.debug {
		log.Printf(str, args...)
	}
}

func ShortImageID(imageID string) string {
	re := regexp.MustCompile(`(sha256:)?([0-9a-zA-Z]{12}).*`)
	return re.ReplaceAllString(imageID, `$2`)
}

// StripImageTagHost strips the host from an image tag
func StripImageTagHost(imageTag string) string {
	re := regexp.MustCompile(`(.*\..*?\/)?(.*)`)
	matches := re.FindStringSubmatch(imageTag)
	imageTag = matches[len(matches)-1]
	imageTag = strings.TrimPrefix(imageTag, "library/")
	return imageTag
}
