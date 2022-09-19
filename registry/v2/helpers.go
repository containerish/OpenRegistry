package registry

import (
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

func (r *registry) getDownloadableURLFromDFSLink(s string) string {
	return fmt.Sprintf("%s/%s", r.config.DFS.S3Any.DFSLinkResolver, s)
}
