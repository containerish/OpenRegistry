package mock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	types "github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
)

type memMappedMockStorage struct {
	memFs           afero.Fs
	logger          telemetry.Logger
	uploadSession   map[string]string
	config          *config.S3CompatibleDFS
	mutex           *sync.RWMutex
	serviceEndpoint string
}

func newMemMappedMockStorage(
	env config.Environment,
	hostAddr string,
	cfg *config.S3CompatibleDFS,
	logger telemetry.Logger,
) dfs.DFS {
	if env != config.CI && env != config.Local {
		panic("mock storage should only be used for CI or Local environments")
	}

	parsedHost, err := url.Parse(hostAddr)
	if err != nil {
		color.Red("error parsing registry endpoint: %s", err)
		os.Exit(1)
	}

	mocker := &memMappedMockStorage{
		memFs:           afero.NewMemMapFs(),
		uploadSession:   make(map[string]string),
		config:          cfg,
		serviceEndpoint: net.JoinHostPort(parsedHost.Hostname(), "5002"),
		logger:          logger,
		mutex:           &sync.RWMutex{},
	}

	go mocker.FileServer()
	return mocker
}

func (ms *memMappedMockStorage) CreateMultipartUpload(layerKey string) (string, error) {
	sessionId := uuid.NewString()
	ms.mutex.Lock()
	ms.uploadSession[sessionId] = sessionId
	defer ms.mutex.Unlock()
	return sessionId, nil
}

func (ms *memMappedMockStorage) UploadPart(
	ctx context.Context,
	uploadId string,
	layerKey string,
	digest string,
	partNumber int32,
	content io.ReadSeeker,
	contentLength int64,
) (s3types.CompletedPart, error) {
	if partNumber > config.MaxS3UploadParts {
		return s3types.CompletedPart{}, errors.New("ERR_TOO_MANY_PARTS")
	}

	if err := ms.validateLayerPrefix(layerKey); err != nil {
		return s3types.CompletedPart{}, err
	}

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
		PartNumber:     aws.Int32(partNumber),
	}, nil
}

func (ms *memMappedMockStorage) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	ms.mutex.Lock()
	delete(ms.uploadSession, layerKey)
	defer ms.mutex.Unlock()
	return layerKey, nil
}

func (ms *memMappedMockStorage) Upload(ctx context.Context, identifier, digest string, content []byte) (string, error) {
	if err := ms.validateLayerPrefix(identifier); err != nil {
		return "", err
	}

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

func (ms *memMappedMockStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	fd, err := ms.memFs.Open(path)
	if err != nil {
		return nil, err
	}

	return fd, nil
}

func (ms *memMappedMockStorage) DownloadDir(dfsLink, dir string) error {
	return nil
}

func (ms *memMappedMockStorage) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (ms *memMappedMockStorage) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

func (ms *memMappedMockStorage) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	var (
		fd  afero.File
		err error
	)

	identifier := types.GetLayerIdentifier(layer.ID)
	parts := strings.Split(identifier, "/")
	if len(parts) > 1 {
		fd, err = ms.memFs.Open(parts[1])
		if err != nil {
			fd, err = ms.memFs.Open(identifier)
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

func (ms *memMappedMockStorage) validateLayerPrefix(identifier string) error {
	if len(identifier) <= LayerKeyPrefixLen || identifier[0:LayerKeyPrefixLen] != LayerKeyPrefix {
		return fmt.Errorf(
			"invalid layer prefix. Found: %s, expected: %s",
			identifier[0:LayerKeyPrefixLen],
			LayerKeyPrefix,
		)
	}

	return nil
}

func (ms *memMappedMockStorage) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	if err := ms.validateLayerPrefix(identifier); err != nil {
		return nil, err
	}

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

func (ms *memMappedMockStorage) getServiceEndpoint() string {
	return fmt.Sprintf("http://%s", ms.serviceEndpoint)
}

func (ms *memMappedMockStorage) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	parts := strings.Split(key, "/")
	if len(parts) == 1 {
		key = "layers/" + key
	}

	preSignedURL := fmt.Sprintf("%s/%s", ms.getServiceEndpoint(), key)
	return preSignedURL, nil
}

func (ms *memMappedMockStorage) AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error {
	if err := ms.validateLayerPrefix(layerKey); err != nil {
		return err
	}

	if err := ms.memFs.Remove(layerKey); err != nil {
		return err
	}

	ms.mutex.Lock()
	delete(ms.uploadSession, uploadId)
	defer ms.mutex.Unlock()
	return nil
}

func (ms *memMappedMockStorage) Config() *config.S3CompatibleDFS {
	return ms.config
}

func (ms *memMappedMockStorage) FileServer() {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Add(http.MethodGet, "/:uuid", func(ctx echo.Context) error {
		fileID := ctx.Param("uuid")
		if err := ms.validateLayerPrefix(fileID); err != nil {
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		fd, err := ms.memFs.Open(fileID)
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})

			ms.logger.Log(ctx, err).Send()
			return echoErr
		}

		bz, _ := io.ReadAll(fd)
		fd.Close()
		echoErr := ctx.Blob(http.StatusOK, "", bz)
		ms.logger.Log(ctx, err).Send()
		return echoErr
	})

	color.Yellow("Started Mock MemMapped DFS on %s", ms.serviceEndpoint)
	if err := e.Start(ms.serviceEndpoint); err != nil {
		color.Red("MockStorage service failed: %s", err)
		os.Exit(1)
	}
}
