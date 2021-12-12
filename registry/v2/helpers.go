package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
		r.logger.Error(e.Error())
	}

	return bz
}

func digest(bz []byte) string {
	hash := sha256.New()
	_, err := hash.Write(bz)
	if err != nil {
		panic(err)
	}

	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func (r *registry) getHttpUrlFromSkylink(s string) string {
	link := strings.TrimPrefix(s, "sia://")
	return fmt.Sprintf("https://siasky.net/%s", link)
}
