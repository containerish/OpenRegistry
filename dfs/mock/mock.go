package mock

import (
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/fatih/color"
)

func NewMockStorage(env config.Environment, hostAddr string, cfg *config.S3CompatibleDFS) dfs.DFS {
	switch cfg.Type {
	case config.MockStorageBackendFileBased:
		color.Green("Storage backend: Mock file-based storage")
		return newFileBasedMockStorage(env, hostAddr, cfg)
	case config.MockStorageBackendMemMapped:
		color.Green("Storage backend: Mock in-memory storage")
		return newMemMappedMockStorage(env, hostAddr, cfg)
	default:
		color.Green("Storage backend: Mock in-memory storage")
		return newMemMappedMockStorage(env, hostAddr, cfg)
	}
}
