package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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
		lm := make(logMsg)
		lm["error"] = e.Error()
		r.debugf(lm)
	}

	return bz
}

func (r *registry) getDigestFromURI(u *url.URL) (string, *RegistryError) {

	elem := strings.Split(u.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		return "", &RegistryError{
			Code:    RegistryErrorCodeNameInvalid,
			Message: "blobs must be attached to a repo",
			Detail:  map[string]interface{}{},
		}
	}

	return elem[len(elem)-1], nil
}

func digest(bz []byte) string {
	hash := sha256.New()
	_, err := hash.Write(bz)
	if err != nil {
		panic(err)
	}

	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func (r *registry) debugf(lm logMsg) {

	if r.debug {
		r.echoLogger.Debug(lm)
	}

	if r.debug {
		e := r.log.Debug()
		e.Fields(lm).Send()
	}
}

func (r *registry) getHttpUrlFromSkylink(s string) string {
	link := strings.TrimPrefix(s, "sia://")
	return fmt.Sprintf("https://skyportal.xyz/%s", link)
}

