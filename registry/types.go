package registry

import (
	"github.com/jay-dee7/register-d/docker"
	"github.com/jay-dee7/register-d/skynet"
)


type Registry struct {
	dockerLocalRegistryHost string
	dockerClient            *docker.Client
	skynetClient            *skynet.Client
	debug                   bool
}

// Config is the config for the registry
type Config struct {
	DockerLocalRegistryHost string
	Skynethost                string
	SkynetGateway             string
	Debug                   bool
}
