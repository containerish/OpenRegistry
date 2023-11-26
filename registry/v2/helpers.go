package registry

import (
	"context"
	"fmt"
)

func (r *registry) getDownloadableURLFromDFSLink(s string) (string, error) {
	presignedUrl, err := r.dfs.GeneratePresignedURL(context.Background(), s)
	if err != nil {
		return "", fmt.Errorf("DFS_ERR_GENERATE_PRESIGNED_URL: %w", err)
	}

	return presignedUrl, nil
}
