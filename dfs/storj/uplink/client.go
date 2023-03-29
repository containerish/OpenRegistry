package uplink

import (
	"context"
	"fmt"

	"github.com/containerish/OpenRegistry/config"
	"storj.io/uplink"
)

func newUplinkClient(cfg *config.Storj) (*uplink.Project, error) {
	access, err := uplink.ParseAccess(cfg.AccessGrantToken)
	if err != nil {
		return nil, fmt.Errorf("ERR_STORJ_UPLINK_PARSE_ACCESS_GRANT_TOKEN: %w", err)
	}

	// edge.JoinShareURL(baseURL string, accessKeyID string, bucket string, key string, *edge.ShareURLOptions{})

	project, err := uplink.OpenProject(context.Background(), access)
	if err != nil {
		return nil, fmt.Errorf("ERR_STORJ_UPLINK_OPEN_PROJECT: %w", err)
	}

	return project, nil
}
