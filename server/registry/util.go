package registry

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func getContent(portal, skynetLink string, s []string) ([]byte, error) {
	uri := skynetURL(portal, s)

	resp, err := http.DefaultClient.Get(uri)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skynetLink: %s, code: %d", skynetLink, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func skynetURL(portal string, s []string) string {
	return fmt.Sprintf("%s/%s", portal, strings.Join(s, "/"))
}
