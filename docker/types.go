package docker

import (
	"github.com/docker/docker/client"
)

type (
	Client struct {
		docker *client.Client
		debug  bool
	}

	Config struct {
		Debug bool
	}

	ImageSummary struct {
		ID string
		Tags []string
		Size int64
	}
)
