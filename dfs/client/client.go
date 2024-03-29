package client

import (
	"log"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/dfs/filebase"
	"github.com/containerish/OpenRegistry/dfs/ipfs/p2p"
	"github.com/containerish/OpenRegistry/dfs/mock"
	"github.com/containerish/OpenRegistry/dfs/storj"
	"github.com/containerish/OpenRegistry/dfs/storj/uplink"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
)

// New returns the first enabled DFS for OpenRegistry.
// It tries for all the possible backends and returns the first one that's enabled.
func New(env config.Environment, registryEndpoint string, cfg *config.DFS, logger telemetry.Logger) dfs.DFS {
	if cfg.Filebase.Enabled {
		color.Green("Storage backend: Filebase")
		return filebase.New(env, &cfg.Filebase)
	}

	if cfg.Storj.Enabled && cfg.Storj.Type == "s3" {
		color.Green("Storage backend: Storj with S3 Gateway")
		return storj.New(env, cfg.Storj.S3Config())
	}

	if cfg.Storj.Enabled && cfg.Storj.Type == "uplink" {
		color.Green("Storage backend: Storj with Uplink")
		return uplink.New(env, &cfg.Storj)
	}

	if cfg.Ipfs.Enabled {
		color.Green("Storage backend: IPFS in P2P mode")
		return p2p.New(&cfg.Ipfs)
	}

	if cfg.Mock.Enabled {
		return mock.NewMockStorage(env, registryEndpoint, &cfg.Mock, logger)
	}

	log.Fatalln(color.RedString("no supported storage backend is enabled"))
	return nil
}
