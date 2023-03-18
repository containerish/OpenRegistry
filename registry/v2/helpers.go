package registry

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
)

func (r *registry) errorResponse(code, msg string, detail map[string]interface{}) []byte {
	var err RegistryErrors

	err.Errors = append(err.Errors, RegistryError{
		Code:    code,
		Message: msg,
		Detail:  detail,
	})

	bz, e := json.Marshal(err)
	if e != nil {
		color.Red("error marshalling error response: %w", err)
	}

	return bz
}

func (r *registry) getDownloadableURLFromDFSLink(s string) (string, error) {
	presignedUrl, err := r.dfs.GeneratePresignedURL(context.Background(), s)
	if err != nil {
		return "", fmt.Errorf("DFS_ERR_GENERATE_PRESIGNED_URL: %w", err)
	}

	return presignedUrl, nil
}
