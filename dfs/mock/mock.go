package mock

import (
	"github.com/fatih/color"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/telemetry"
)

func NewMockStorage(
	env config.Environment,
	hostAddr string,
	cfg *config.S3CompatibleDFS,
	logger telemetry.Logger,
) dfs.DFS {
	switch cfg.Type {
	case config.MockStorageBackendFileBased:
		color.Green("Storage backend: Mock file-based storage")
		return newFileBasedMockStorage(env, hostAddr, cfg, logger)
	case config.MockStorageBackendMemMapped:
		color.Green("Storage backend: Mock in-memory storage")
		return newMemMappedMockStorage(env, hostAddr, cfg, logger)
	default:
		color.Green("Storage backend: Mock in-memory storage")
		return newMemMappedMockStorage(env, hostAddr, cfg, logger)
	}
}

const (
	MockFSPath        = ".mock-fs"
	LayerKeyPrefix    = "layers"
	LayerKeyPrefixLen = len(LayerKeyPrefix) // to account for trailing slash
)
