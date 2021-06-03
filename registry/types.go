package registry

import (
	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/docker"
	"github.com/jay-dee7/parachute/skynet"
)

type Registry struct {
	c                       *config.RegistryConfig
	dockerLocalRegistryHost string
	dockerClient            *docker.Client
	skynetClient            *skynet.Client
	debug                   bool
}
