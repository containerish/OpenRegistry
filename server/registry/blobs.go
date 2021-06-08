package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/fatih/color"
)

func isBlob(r *http.Request) bool {
	el := strings.Split(r.URL.Path, "/")
	el = el[1:]

	if el[len(el)-1] == "" {
		el = el[:len(el)-1]
	}

	if len(el) < 3 {
		return false
	}

	return el[len(el)-2] == "blobs" || (el[len(el)-3] == "blobs" &&
		el[len(el)-2] == "uploads")
}

type blobs struct {
	contents map[string][]byte
	uploads  map[string][]byte
	lock     sync.Mutex

	layers   map[string][]string
	registry *registry
}

func (b *blobs) handle(rw http.ResponseWriter, req *http.Request) *restError {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		return &restError{
			Status:  http.StatusBadRequest,
			Code:    "NAME_INVALID",
			Message: "blobs must be attached to a repo",
		}
	}

	target := elem[len(elem)-1]
	service := elem[len(elem)-2]
	digest := req.URL.Query().Get("digest")
	contentRange := req.Header.Get("Content-Range")
	repo := strings.Join(elem[1:len(elem)-2], "/")

	if service == "uploads" {
		repo = strings.Join(elem[1:len(elem)-3], "/")
	}

	if req.Method == "HEAD" {
		return b.handleHead(rw, repo, target)
	}

	if req.Method == http.MethodGet {
		return b.handleGet(rw, repo, target)
	}

	if req.Method == http.MethodPost && target == "uploads" && digest != "" {
		return b.handlePost(rw, req, repo, digest)
	}

	if req.Method == "POST" && target == "uploads" && digest == "" {
		id := fmt.Sprint(rand.Int63())
		rw.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-2]...), "blobs/uploads", id))
		rw.Header().Set("Range", "0-0")
		rw.WriteHeader(http.StatusAccepted)
		return nil
	}

	if req.Method == http.MethodPatch && service == "uploads" && contentRange != "" {
		return b.handlePatch(rw, req, target, contentRange, elem)
	}

	if req.Method == "PATCH" && service == "uploads" && contentRange == "" {
		b.lock.Lock()
		defer b.lock.Unlock()
		if _, ok := b.uploads[target]; ok {
			return &restError{
				Status:  http.StatusBadRequest,
				Code:    "BLOB_UPLOAD_INVALID",
				Message: "Stream uploads after first write are not allowed",
			}
		}

		l := &bytes.Buffer{}
		io.Copy(l, req.Body)

		b.uploads[target] = l.Bytes()
		rw.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-3]...), "blobs/uploads", target))
		rw.Header().Set("Range", fmt.Sprintf("0-%d", len(l.Bytes())-1))
		rw.WriteHeader(http.StatusNoContent)
		return nil
	}

	if req.Method == "PUT" && service == "uploads" && digest == "" {
		return &restError{
			Status:  http.StatusBadRequest,
			Code:    "DIGEST_INVALID",
			Message: "digest not specified",
		}
	}

	if req.Method == "PUT" && service == "uploads" && digest != "" {
		b.lock.Lock()
		defer b.lock.Unlock()
		l := bytes.NewBuffer(b.uploads[target])
		io.Copy(l, req.Body)
		rd := sha256.Sum256(l.Bytes())
		d := "sha256:" + hex.EncodeToString(rd[:])
		if d != digest {
			return &restError{
				Status:  http.StatusBadRequest,
				Code:    "DIGEST_INVALID",
				Message: "digest does not match contents",
			}
		}

		b.contents[d] = l.Bytes()
		digests := b.layers[repo]
		b.layers[repo] = append(digests, d)
		delete(b.uploads, target)
		rw.Header().Set("Docker-Content-Digest", d)
		rw.WriteHeader(http.StatusCreated)
		return nil
	}

	return &restError{
		Status:  http.StatusBadRequest,
		Code:    "METHOD_UNKNOWN",
		Message: "handle eror",
	}
}

func (b *blobs) handleHead(rw http.ResponseWriter, repo, target string) *restError {
	b.lock.Lock()
	defer b.lock.Unlock()

	// content is available if image is locally pushed
	if c, ok := b.contents[target]; ok {
		rw.Header().Set("Content-Length", fmt.Sprint(len(c)))
		rw.Header().Set("Docker-Content-Digest", target)
		rw.WriteHeader(http.StatusOK)
		return nil
	}

	skynetlink, err := b.registry.resolveSkynetLink(repo, target)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "ERR_RESOLVE_LINK",
			Message: err.Error(),
		}
	}

	uri := b.registry.skynetURL([]string{skynetlink, "blobs", target})
	resp, err := http.Head(uri)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: err.Error(),
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: resp.Status,
		}
	}

	bz, err := io.ReadAll(resp.Body)
	size := len(bz)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: err.Error(),
		}
	}

	rw.Header().Set("Content-Length", fmt.Sprint(size))
	rw.Header().Set("Docker-Content-Digest", target)
	rw.WriteHeader(resp.StatusCode)
	io.CopyN(rw, bytes.NewReader(bz), int64(size))

	return nil
}

func (b *blobs) handleGet(rw http.ResponseWriter, repo, target string) *restError {
	skynetlink, err := b.registry.resolveSkynetLink(repo, target)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: err.Error(),
		}
	}

	uri := b.registry.skynetURL([]string{skynetlink, "blobs", target})
	color.Red("%s - %s\n - %s\n", skynetlink, target, uri)

	resp, err := http.DefaultClient.Get(uri)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "GET_REQUEST_FAILED",
			Message: err.Error(),
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: resp.Status,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "BLOB_UNKNOWN",
			Message: err.Error(),
		}
	}
	size := len(body)
	rw.Header().Set("Content-Length", fmt.Sprint(size))
	rw.Header().Set("Docker-Content-Digest", target)
	rw.WriteHeader(resp.StatusCode)
	io.CopyN(rw, bytes.NewReader(body), int64(size))
	return nil
}

func (b *blobs) handlePost(rw http.ResponseWriter, req *http.Request, repo, digest string) *restError {
	l := &bytes.Buffer{}
	io.Copy(l, req.Body)
	rd := sha256.Sum256(l.Bytes())
	d := "sha256:" + hex.EncodeToString(rd[:])
	if d != digest {
		return &restError{
			Status:  http.StatusBadRequest,
			Code:    "DIGEST_INVALID",
			Message: "digest does not match contents",
		}
	}

	fmt.Println("sha256 ok")

	b.lock.Lock()
	defer b.lock.Unlock()

	b.contents[d] = l.Bytes()
	digests := b.layers[repo]
	b.layers[repo] = append(digests, d)
	rw.Header().Set("Docker-Content-Digest", d)
	rw.WriteHeader(http.StatusCreated)
	return nil
}

func (b *blobs) handlePatch(rw http.ResponseWriter, req *http.Request, target, contentRange string, elem []string) *restError {
	start, end := 0, 0
	if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
		return &restError{
			Status:  http.StatusRequestedRangeNotSatisfiable,
			Code:    "BLOB_UPLOAD_UNKNOWN",
			Message: "handle path error: " + err.Error(),
		}
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	if start != len(b.uploads[target]) {
		return &restError{
			Status:  http.StatusRequestedRangeNotSatisfiable,
			Code:    "BLOB_UPLOAD_UNKNOWN",
			Message: "Your content range doesn't match what we have",
		}
	}
	l := bytes.NewBuffer(b.uploads[target])
	io.Copy(l, req.Body)
	b.uploads[target] = l.Bytes()
	rw.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-3]...), "blobs/uploads", target))
	rw.Header().Set("Range", fmt.Sprintf("0-%d", len(l.Bytes())-1))
	rw.WriteHeader(http.StatusNoContent)
	return nil
}

func (b *blobs) remove(repo string) {
	b.lock.Lock()

	digests, ok := b.layers[repo]
	if !ok {
		return
	}
	delete(b.layers, repo)

	for _, d := range digests {
		delete(b.contents, d)
	}

	b.lock.Unlock()
}

func (b *blobs) get(repo string) (map[string][]byte, bool) {
	b.lock.Lock()

	digests, ok := b.layers[repo]
	if !ok {
		return nil, false
	}

	layers := make(map[string][]byte)
	for _, d := range digests {
		blob, ok := b.contents[d]
		if !ok {
			return nil, false
		}
		layers[d] = blob
	}

	b.lock.Unlock()
	return layers, true
}
