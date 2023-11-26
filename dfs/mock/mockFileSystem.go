package mock

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	types "github.com/containerish/OpenRegistry/store/v1/types"
	core_types "github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"
)

type fileBasedMockStorage struct {
	fs              afero.Fs
	uploadSession   map[string]string
	config          *config.S3CompatibleDFS
	hostAddr        string
	serviceEndpoint string
}

func newFileBasedMockStorage(env config.Environment, hostAddr string, cfg *config.S3CompatibleDFS) dfs.DFS {
	if env != config.CI && env != config.Local {
		panic("mock storage should only be used for CI or Local environments")
	}

	parsedHost, err := url.Parse(hostAddr)
	if err != nil {
		color.Red("error parsing registry endpoint: %s", err)
		os.Exit(1)
	}

	_ = os.MkdirAll(".mock-fs", os.ModePerm)
	mocker := &fileBasedMockStorage{
		fs:              afero.NewBasePathFs(afero.NewOsFs(), ".mock-fs"),
		uploadSession:   make(map[string]string),
		config:          cfg,
		serviceEndpoint: "0.0.0.0:8080",
		hostAddr:        parsedHost.Hostname(),
	}

	go mocker.FileServer()
	return mocker
}

func (ms *fileBasedMockStorage) CreateMultipartUpload(layerKey string) (string, error) {
	sessionId := uuid.NewString()
	ms.uploadSession[sessionId] = sessionId
	return sessionId, nil
}

func (ms *fileBasedMockStorage) UploadPart(
	ctx context.Context,
	uploadId string,
	layerKey string,
	digest string,
	partNumber int64,
	content io.ReadSeeker,
	contentLength int64,
) (s3types.CompletedPart, error) {
	idParts := strings.Split(layerKey, "/")
	if len(idParts) > 1 {
		_ = ms.fs.MkdirAll(strings.Join(idParts[:len(idParts)-1], "/"), os.ModePerm)
	}
	fd, err := ms.fs.OpenFile(layerKey, os.O_RDWR|os.O_CREATE, os.ModePerm)

	if err != nil {
		return s3types.CompletedPart{}, err
	}

	bz, _ := io.ReadAll(content)
	if _, err = fd.Write(bz); err != nil {
		return s3types.CompletedPart{}, err
	}
	if err = fd.Sync(); err != nil {
		return s3types.CompletedPart{}, err
	}
	fd.Close()

	return s3types.CompletedPart{
		ChecksumCRC32:  &digest,
		ChecksumCRC32C: &layerKey,
		PartNumber:     aws.Int32(int32(partNumber)),
	}, nil
}

func (ms *fileBasedMockStorage) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	delete(ms.uploadSession, layerKey)
	return layerKey, nil
}

func (ms *fileBasedMockStorage) Upload(ctx context.Context, identifier, digest string, content []byte) (string, error) {
	idParts := strings.Split(identifier, "/")
	if len(idParts) > 1 {
		_ = ms.fs.MkdirAll(strings.Join(idParts[:len(idParts)-1], "/"), os.ModePerm)
	}
	fd, err := ms.fs.Create(identifier)
	if err != nil {
		return "", err
	}

	if _, err = fd.Write(content); err != nil {
		return "", err
	}
	if err = fd.Sync(); err != nil {
		return "", err
	}
	defer fd.Close()

	return identifier, nil
}

func (ms *fileBasedMockStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fd, err := ms.fs.Open(path)
	if err != nil {
		return nil, err
	}

	return fd, nil
}

func (ms *fileBasedMockStorage) DownloadDir(dfsLink, dir string) error {
	return nil
}

func (ms *fileBasedMockStorage) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (ms *fileBasedMockStorage) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

func (ms *fileBasedMockStorage) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	var (
		fd  afero.File
		err error
	)

	identifier := core_types.GetLayerIdentifier(layer.ID)
	parts := strings.Split(identifier, "/")
	if len(parts) > 1 {
		fd, err = ms.fs.Open(parts[1])
		if err != nil {
			fd, err = ms.fs.Open(identifier)
		}
	}
	if err != nil {
		return nil, err
	}

	stat, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	fd.Close()

	return &types.ObjectMetadata{
		ContentType:   "",
		Etag:          "",
		DFSLink:       identifier,
		ContentLength: int(stat.Size()),
	}, nil

}

func (ms *fileBasedMockStorage) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	fd, err := ms.fs.Open(identifier)
	if err != nil {
		return nil, err
	}

	stat, err := fd.Stat()
	if err != nil {
		return nil, err
	}

	return &types.ObjectMetadata{
		ContentLength: int(stat.Size()),
	}, nil
}

func (ms *fileBasedMockStorage) getServiceEndpoint() string {
	_, port, err := net.SplitHostPort(ms.serviceEndpoint)
	if err != nil {
		color.Red("error splitting mock service host port: %s", err)
		os.Exit(1)
	}

	return fmt.Sprintf("http://%s:%s", ms.hostAddr, port)
}

func (ms *fileBasedMockStorage) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, "/")
	if len(parts) == 1 {
		key = "layers/" + key
	}

	preSignedURL := fmt.Sprintf("%s/%s", ms.getServiceEndpoint(), key)
	return preSignedURL, nil
}

func (ms *fileBasedMockStorage) AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error {
	if err := ms.fs.Remove(layerKey); err != nil {
		return err
	}

	delete(ms.uploadSession, uploadId)
	return nil
}

func (ms *fileBasedMockStorage) Config() *config.S3CompatibleDFS {
	return ms.config
}

func (ms *fileBasedMockStorage) FileServer() {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Add(http.MethodGet, "/:uuid", func(ctx echo.Context) error {
		fileID := ctx.Param("uuid")
		fd, err := ms.fs.Open(fileID)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}

		bz, _ := io.ReadAll(fd)
		fd.Close()
		return ctx.Blob(http.StatusOK, "", bz)
	})

	if err := e.Start(ms.serviceEndpoint); err != nil {
		color.Red("MockStorage service failed: %s", err)
		os.Exit(1)
	}
}