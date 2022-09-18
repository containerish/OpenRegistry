package registry

import (
	"encoding/json"
	"fmt"
	"strings"

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

func (r *registry) getHttpUrlFromSkylink(s string) string {
	link := strings.TrimPrefix(s, "sia://")
	return fmt.Sprintf("https://siasky.net/%s", link)
}

func (r *registry) getDownloadableURLFromDFSLink(s string) string {
	return fmt.Sprintf("https://ipfs.filebase.io/ipfs/%s", s)
}
