package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jay-dee7/register-d/docker"
	"github.com/jay-dee7/register-d/skynet"
)

func New(c *Config) *Registry {
	if c == nil {
		c = &Config{}
	}

	dockerLocalRegistryHost := c.DockerLocalRegistryHost

	if dockerLocalRegistryHost == "" {
		dockerLocalRegistryHost = os.Getenv("DOCKER_LOCAL_REGISTRY_HOST")
		if dockerLocalRegistryHost == "" {
			dockerLocalRegistryHost = "0.0.0.0"
		}
	}

	skynetClient := skynet.NewClient(&skynet.Config{
		Host:       c.Skynethost,
		GatewayURL: c.SkynetGateway,
	})

	dockerClient := docker.New(&docker.Config{})

	return &Registry{
		dockerLocalRegistryHost: dockerLocalRegistryHost,
		dockerClient:            dockerClient,
		skynetClient:            skynetClient,
		debug:                   false,
	}
}

func (r *Registry) PushImagebyID(imageID string) (string, error) {
	id, err := r.TagToImageID(imageID)
	if err != nil {
		return "", err
	}

	reader, err := r.dockerClient.ReadImage(id)
	if err != nil {
		return "", err
	}

	return r.PushImage(reader, imageID)
}

func (r *Registry) TagToImageID(imageID string) (string, error) {
	images, err := r.dockerClient.ListImages()
	if err != nil {
		return "", err
	}

	for _, i := range images {
		if strings.HasPrefix(i.ID, imageID) {
			break
		}

		for _, tag := range i.Tags {
			if tag == imageID || tag == imageID+":latest" {
				imageID = i.ID
			}
		}
	}

	return imageID, nil
}

func (r *Registry) PushImage(reader io.Reader, imageID string) (string, error) {
	tmp := os.TempDir()

	if err := r.untar(reader, tmp); err != nil {
		return "", err
	}

	dirPath, err := r.dirSetup(tmp, imageID)
	if err != nil {
		return "", err
	}

	return r.uploadDir(dirPath)
}

func (r *Registry) untar(reader io.Reader, dst string) error {
	tr := tar.NewReader(reader)

	for {
		h, err := tr.Next()
		switch{
		case err == io.EOF :
			return nil
		case err != nil:
			return err
		case h == nil:
			continue
		}

		target := filepath.Join(dst, h.Name)

		switch h.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
			case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy contents to file
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
}

func (r *Registry) uploadDir(dirPath string) (string, error) {
	skylink, err := r.skynetClient.UploadDirectory(dirPath)
	if err != nil {
		return "", err
	}

	skynetMetadata, err := r.skynetClient.List(skylink)
	if err != nil {
		return "", err
	}

	if len(skynetMetadata) < 1 {
		return "", fmt.Errorf("could not upload")
	}

	return skynetMetadata[0].SkyLink, nil
}

func (r *Registry) Debugf(s string, args ...interface{}) {
	if r.debug {
		log.Printf(s, args...)
	}
}

func (r *Registry) writeJSON(data interface{}, path string) error{
	bz, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return os.WriteFile(path, bz, os.ModePerm)
}

func (r *Registry) makeV2Manifest(manifest map[string]interface{}, configDigest, configDest, tmp, workdir string) (map[string]interface{}, error) {
	v2Manifest, err := r.prepareV2Manifest(manifest, tmp, workdir+"/blobs")
	if err != nil {
		return nil, err
	}

	config := make(map[string]interface{})
	config["digest"] = configDigest
	config["size"], err = r.fileSize(configDest)
	if err != nil {
		return nil, err
	}

	config["mediaType"] = "application/vnd.docker.container.image.v1+json"
	conf, ok := v2Manifest["config"].(map[string]interface{})
	if !ok {
		return nil, err
	}

	v2Manifest["config"] = r.mergeMaps(conf, config)
	return v2Manifest, nil
}

func (r *Registry) prepareV2Manifest(manifest map[string]interface{}, tmp, blobDir string) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	res["schemaVersion"] = 2
	res["mediaType"] = "application/vnd.docker.distribution.manifest.v2+json"

	config := make(map[string]interface{})
	res["config"] = config

	var layers []map[string]interface{}

	ls, ok := manifest["Layers"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected layers")
	}

	mediaType := "application/vnd.docker.image.rootfs.diff.tar.gzip"
	for _, iface := range ls {
		l, ok := iface.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}

		o := make(map[string]interface{})
		o["mediaType"] = mediaType
		tmpPath := fmt.Sprintf("%s/%s", tmp, l)
		size, digest, err := r.compressLayer(tmpPath, blobDir)
		if err != nil {
			return nil, err
		}

		o["size"] = size
		o["digest"] = digest
		layers = append(layers, o)
	}

	res["layers"] = layers

	return res, nil
}

func (r *Registry) compressLayer(path, blobDir string) (int64, string, error) {
	tmp := fmt.Sprintf("%s/layer/tmp.tgz", path)

	if err := r.gzip(path, tmp); err != nil {
		return int64(0), "", err
	}

	digest, err := r.sha256(tmp)
	if err != nil {
		return int64(0), "", err
	}

	size, err := r.fileSize(tmp)
	if err != nil {
		return int64(0), "", err
	}

	if err = r.renameFile(tmp, fmt.Sprintf("%s/sha256:%s", blobDir, digest)); err != nil {
		return int64(0), "", err
	}

	return size, digest, nil

}

func (r *Registry) gzip(src, dst string) error {
	bz, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	writer.Write(bz)
	writer.Close()

	return os.WriteFile(dst, buf.Bytes(), os.ModePerm)
}

func (r *Registry) sha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()

	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (r *Registry) fileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return int64(0), err
	}

	return fi.Size(), nil
}

func (r *Registry) renameFile(src, dst string) error {
	return os.Rename(src, dst)
}

func (r *Registry) mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	for k, v := range b {
		a[k] = v
	}

	return a
}

func (r *Registry) dirSetup(tmp, imageID string) (string, error) {
	root := os.TempDir()
	workdir := root
	name := "default"

	if _, err := os.Stat(tmp + "repositories"); err != nil {
		repos, err := r.readJSON(tmp + "/repositories")
		if err != nil {
			return "", err
		}
		if len(repos) != 1 {
			return "", fmt.Errorf("only one repo expected")
		}

		for imageName, tags := range repos {
			if len(tags) != 1 {
				return "", fmt.Errorf("only one tag expected")
			}

			for range tags {
				name = r.sanitizeImageName(imageName)
			}
		}
	}

	r.createDirs(workdir+name)

	manifestJSON, err := r.readJSONArray(tmp+"/manifest.json")
	if err != nil {
		return "", err
	}

	if len(manifestJSON) == 0 {
		return "", fmt.Errorf("expected manifest to contain data")
	}

	m := manifestJSON[0]

	configFile, ok := m["Config"].(string)
	if !ok {
		return "", fmt.Errorf("image archive must be produced by docker")
	}

	configDigest := fmt.Sprintf("sha256:%s", configFile[:len(configFile)-5])
	configDest := fmt.Sprintf("%s/blobs/%s", workdir, configDigest)

	src := fmt.Sprintf("%s/%s", configFile, configDest)
	bz, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}

	os.WriteFile(configDest, bz, os.ModePerm)

	mf, err := r.makeV2Manifest(m, configDest, configDest, tmp, workdir)
	if err != nil {
		return "", err
	}

	ref := func(s string) string {
		if strings.Contains(s, "sha256:") {
			return "'latest"
		}
		ss := strings.SplitN(s, ":", 2)
		if len(ss) == 2 {
			return ss[1]
		}

		return "latest"
	}

	writeManifest := func() error {
		tag := ref(imageID)
		if tag != "latest" {
			if err = r.writeJSON(mf, workdir + "/manifests/latest"); err != nil {
				return err
			}
		}

		bz, err := json.Marshal(mf)
		if err != nil {
			return err
		}

		rd := sha256.Sum256(bz)

		fullPath := fmt.Sprintf("%s/manifests.sha256:%s", workdir, hex.EncodeToString(rd[:]))

		return r.writeJSON(mf, fullPath)
	}

	if err = writeManifest(); err != nil {
		return "", err
	}

	return root, nil

}

func (r *Registry) createDirs(workdir string) {
	os.MkdirAll(workdir, os.ModePerm)
	os.MkdirAll(workdir + "/manifests", os.ModePerm)
	os.MkdirAll(workdir + "/blobs", os.ModePerm)
}

func (r *Registry) readJSON(path string) (map[string]map[string]string, error) {
	bz, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	data := make(map[string]map[string]string)

	if err = json.Unmarshal(bz, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (r *Registry) readJSONArray(path string) ([]map[string]interface{}, error) {
	bz, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data []map[string]interface{}
	if err = json.Unmarshal(bz, &data); err != nil {
		return data, err
	}

	return data, nil
}

func (r *Registry) sanitizeImageName(imageID string) string {
	return imageID
}
