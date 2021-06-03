package registry

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type skynetStore struct {
	skynetLinks map[string]string
	location string
	sync.RWMutex
}

func newSkynetStore(location string) *skynetStore {
	return &skynetStore{
		skynetLinks: map[string]string{},
		location:    location,
	}
}

func key(repo, ref string) string {
	return repo + ":" + ref
}

func (s *skynetStore) Add(repo, ref, skynetLink string) {
	s.Lock()
	defer s.Unlock()

	k := key(repo, ref)

	s.skynetLinks[k] = skynetLink

	if repo != skynetLink && !strings.HasPrefix(ref, "sha256:") {
		s.writeSkynetLink(k, skynetLink)
	}
}

func (s *skynetStore) Get(repo, ref string) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	k := key(repo, ref)

	val, ok := s.skynetLinks[k]
	if !ok {
		if v, err := s.readSkynetLink(k); err == nil {
			val = v
			ok = true
		}
	}

	return val, ok
}

func (s *skynetStore) readSkynetLink(key string) (string, error) {
	pc := strings.SplitN(key, ":", 2)
	p := filepath.Join(s.location, strings.Join(pc, "/"))
	content, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (s *skynetStore) writeSkynetLink(key, val string) error {
	pc := strings.SplitN(key, ":", 2)
	p := filepath.Join(s.location, strings.Join(pc, "/"))
	if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
		return err
	}

	return os.WriteFile(p, []byte(val), 0644)
}
