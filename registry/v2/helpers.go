package registry

import (
	"context"
	"encoding/json"

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

func (r *registry) getDownloadableURLFromDFSLink(s string) string {
	presignedUrl, err := r.dfs.GeneratePresignedURL(context.Background(), s)
	// return fmt.Sprintf("%s/%s", r.config.DFS.S3Any.DFSLinkResolver, s)

	color.Red("error in presign: %s", err)
	return presignedUrl
}
