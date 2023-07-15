package mock

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	skynet "github.com/SkynetLabs/go-skynet/v2"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"
)

func (ms *mockStorage) CreateMultipartUpload(layerKey string) (string, error) {
	sessionId := uuid.NewString()
	ms.uploadSession[sessionId] = sessionId
	return sessionId, nil
}

func (ms *mockStorage) UploadPart(
	ctx context.Context,
	uploadId string,
	layerKey string,
	digest string,
	partNumber int64,
	content io.ReadSeeker,
	contentLength int64,
) (s3types.CompletedPart, error) {
	fd, err := ms.memFs.OpenFile(layerKey, os.O_RDWR|os.O_CREATE, os.ModePerm)
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
		PartNumber:     int32(partNumber),
	}, nil
}

func (ms *mockStorage) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	delete(ms.uploadSession, layerKey)
	return layerKey, nil
}

func (ms *mockStorage) Upload(ctx context.Context, identifier, digest string, content []byte) (string, error) {
	fd, err := ms.memFs.Create(identifier)
	if err != nil {
		return "", err
	}

	bz, _ := io.ReadAll(fd)
	if _, err = fd.Write(bz); err != nil {
		return "", err
	}
	if err = fd.Sync(); err != nil {
		return "", err
	}
	fd.Close()

	return identifier, nil
}

func (ms *mockStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fd, err := ms.memFs.Open(path)
	if err != nil {
		return nil, err
	}

	return fd, nil
}

func (ms *mockStorage) DownloadDir(skynetLink, dir string) error {
	return nil
}

func (ms *mockStorage) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (ms *mockStorage) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

func (ms *mockStorage) Metadata(identifier string) (*skynet.Metadata, error) {
	fd, err := ms.memFs.Open(identifier)
	if err != nil {
		return nil, err
	}

	stat, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	fd.Close()

	return &skynet.Metadata{
		ContentType:   "",
		Etag:          "",
		Skylink:       identifier,
		ContentLength: int(stat.Size()),
	}, nil

}

func (ms *mockStorage) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	fd, err := ms.memFs.Open(identifier)
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

func (ms *mockStorage) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, "/")
	if len(parts) == 1 {
		key = "layers/" + key
	}

	preSignedURL := fmt.Sprintf("http://%s/%s", ms.serverEndpoint, key)
	return preSignedURL, nil
}

func (ms *mockStorage) AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error {
	if err := ms.memFs.Remove(layerKey); err != nil {
		return err
	}

	delete(ms.uploadSession, uploadId)
	return nil
}

func (ms *mockStorage) Config() *config.S3CompatibleDFS {
	return ms.config
}

func (ms *mockStorage) FileServer() {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Add(http.MethodGet, "/:uuid", func(ctx echo.Context) error {
		fileID := ctx.Param("uuid")
		fd, err := ms.memFs.Open(fileID)
		if err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}

		bz, _ := io.ReadAll(fd)
		fd.Close()
		return ctx.Blob(http.StatusOK, "", bz)
	})

	e.Start(ms.serverEndpoint)
}

type mockStorage struct {
	memFs          afero.Fs
	uploadSession  map[string]string
	config         *config.S3CompatibleDFS
	serverEndpoint string
}

func NewMockStorage(env config.Environment, cfg *config.S3CompatibleDFS) dfs.DFS {
	if env != config.CI && env != config.Local {
		panic("mock storage should only be used for CI or Local environments")
	}

	mocker := &mockStorage{
		memFs:          afero.NewMemMapFs(),
		uploadSession:  make(map[string]string),
		config:         cfg,
		serverEndpoint: "0.0.0.0:8080",
	}

	go mocker.FileServer()

	return mocker
}
