package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/jay-dee7/parachute/server/registry/image"
)

type manifest struct {
	contentType string
	blob        []byte
	digest      string // computed
}

type manifests struct {
	// maps repo - manifest tag/digest -> manifest
	manifests map[string]map[string]*manifest
	lock      sync.Mutex

	registry *registry
}

func isManifest(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 4 {
		return false
	}
	return elems[len(elems)-2] == "manifests"
}

// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pulling-an-image-manifest
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pushing-an-image
func (m *manifests) handle(resp http.ResponseWriter, req *http.Request) *restError {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	target := elem[len(elem)-1]
	repo := strings.Join(elem[1:len(elem)-2], "/")

	if req.Method == "GET" {
		m.lock.Lock()
		defer m.lock.Unlock()

		mf, err := m.fetch(repo, target)
		if err != nil {
			return &restError{
				Status:  http.StatusNotFound,
				Code:    "MANIFEST_UNKNOWN",
				Message: err.Error(),
			}
		}

		// Prepare reverse lookup by digest for pulling blobs from skynet
		cid, err := m.registry.resolveSkynetLink(repo, target)
		if err != nil {
			return &restError{
				Status:  http.StatusNotFound,
				Code:    "MANIFEST_UNKNOWN",
				Message: err.Error(),
			}
		}
		f, _ := image.DecodeManifest(mf.blob)

		for _, d := range f.Digests() {
			m.registry.skyneStore.Add(repo, d, cid)
		}

		resp.Header().Set("Docker-Content-Digest", mf.digest)
		resp.Header().Set("X-Docker-Content-ID", cid)
		resp.Header().Set("Content-Type", mf.contentType)
		resp.Header().Set("Content-Length", fmt.Sprint(len(mf.blob)))
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, bytes.NewReader(mf.blob))
		return nil
	}

	if req.Method == "HEAD" {
		m.lock.Lock()
		defer m.lock.Unlock()

		mf, err := m.fetch(repo, target)
		if err != nil {
			return &restError{
				Status:  http.StatusNotFound,
				Code:    "MANIFEST_UNKNOWN",
				Message: err.Error(),
			}
		}

		resp.Header().Set("Docker-Content-Digest", mf.digest)
		resp.Header().Set("Content-Type", mf.contentType)
		resp.Header().Set("Content-Length", fmt.Sprint(len(mf.blob)))
		resp.WriteHeader(http.StatusOK)
		return nil
	}

	if req.Method == "PUT" {
		m.lock.Lock()
		defer m.lock.Unlock()

		if _, ok := m.manifests[repo]; !ok {
			m.manifests[repo] = map[string]*manifest{}
		}
		b := &bytes.Buffer{}
		io.Copy(b, req.Body)

		digest := computeDigest(b.Bytes())
		mf := manifest{
			blob:        b.Bytes(),
			contentType: req.Header.Get("Content-Type"),
		}

		// If the manifest is a manifest list, check that the manifest
		// list's constituent manifests are already uploaded.
		// This isn't strictly required by the registry API, but some
		// registries require this.
		if mf.contentType == string(types.OCIImageIndex) ||
			mf.contentType == string(types.DockerManifestList) {

			im, err := v1.ParseIndexManifest(b)
			if err != nil {
				return &restError{
					Status:  http.StatusNotFound,
					Code:    "MANIFEST_UNKNOWN",
					Message: err.Error(),
				}
			}
			for _, desc := range im.Manifests {
				if _, found := m.manifests[repo][desc.Digest.String()]; !found {
					return &restError{
						Status:  http.StatusNotFound,
						Code:    "MANIFEST_UNKNOWN",
						Message: fmt.Sprintf("Sub-manifest %q not found", desc.Digest),
					}
				}
			}
		}

		// Allow future references by target (tag) and immutable digest.
		// See https://docs.docker.com/engine/reference/commandline/pull/#pull-an-image-by-digest-immutable-identifier.
		m.manifests[repo][target] = &mf
		m.manifests[repo][digest] = &mf

		// layers, ok := m.registry.blobs.get(repo)
		// if !ok {
		// 	return &restError{
		// 		Status:  http.StatusNotFound,
		// 		Code:    "MANIFEST_BLOB_UNKNOWN",
		// 		Message: fmt.Sprintf("layers for %q not found", repo),
		// 	}
		// }

		// // TODO cache e.g. move to disk?
		// m.registry.blobs.remove(repo)

		// refs := make(map[string][]byte)
		// refs[target] = mf.blob
		// refs[digest] = mf.blob
		// refs["latest"] = mf.blob // <cid>/latest

		// cid, err := m.registry.skyentClient.AddImage(refs, layers)
		// if err != nil {
		// 	return &restError{
		// 		Status:  http.StatusInternalServerError,
		// 		Code:    "",
		// 		Message: err.Error(),
		// 	}
		// }

		// color.Red(cid)

		// m.registry.skyneStore.Add(repo, target, cid)
		// m.registry.skyneStore.Add(repo, digest, cid)
		// m.registry.skyneStore.Add(cid, "latest", cid) // <cid>/latest

		// resp.Header().Set("Docker-Content-Digest", digest)
		// resp.Header().Set("X-Docker-Content-ID", cid)
		resp.WriteHeader(http.StatusCreated)
		return nil
	}
	return &restError{
		Status:  http.StatusBadRequest,
		Code:    "METHOD_UNKNOWN",
		Message: "invalid options for method handle",
	}
}

func (m *manifests) fetch(repo, target string) (*manifest, error) {
	if _, ok := m.manifests[repo]; !ok {
		m.manifests[repo] = map[string]*manifest{}
	}
	mf, ok := m.manifests[repo][target]
	if ok {
		return mf, nil
	}

	cid, err := m.registry.resolveSkynetLink(repo, target)
	if err != nil {
		return nil, err
	}

	mf, err = m.getManifest(cid, target)
	if err != nil {
		return nil, err
	}

	m.manifests[repo][target] = mf
	m.manifests[repo][mf.digest] = mf

	// conform to the distribution registry specification
	// in case target is tag, we need to resolve also by hash.
	m.registry.skyneStore.Add(repo, mf.digest, cid)

	return mf, nil
}

func computeDigest(b []byte) string {
	rd := sha256.Sum256(b)
	d := "sha256:" + hex.EncodeToString(rd[:])
	return d
}

func (m *manifests) getManifest(cid, target string) (*manifest, error) {
	b, err := getContent(m.registry.c.SkynetPortalURL, cid, []string{"manifests", target})
	if err != nil {
		return nil, err
	}
	mf, err := image.DecodeManifest(b)
	if err != nil {
		return nil, err
	}
	digest := computeDigest(b)
	return &manifest{
		blob:        b,
		contentType: mf.MediaType,
		digest:      digest,
	}, nil
}
