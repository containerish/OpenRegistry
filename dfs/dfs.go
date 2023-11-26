package dfs

import (
	"context"
	"io"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

type DFS interface {
	Upload(ctx context.Context, namespace, digest string, content []byte) (string, error)
	// MultipartUpload returns uploadid or error
	CreateMultipartUpload(namespace string) (string, error)
	UploadPart(
		ctx context.Context,
		uploadId string,
		key string,
		digest string,
		partNumber int64,
		content io.ReadSeeker,
		contentLength int64,
	) (s3types.CompletedPart, error)

	// ctx is used for handling any request cancellations.
	// @param uploadId: string is the ID of the layer being uploaded
	CompleteMultipartUpload(
		ctx context.Context,
		uploadId string,
		key string,
		finalDigest string,
		completedParts []s3types.CompletedPart,
	) (string, error)
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	DownloadDir(dfsLink, dir string) error
	List(path string) ([]*types.Metadata, error)
	AddImage(ns string, mf, l map[string][]byte) (string, error)
	Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error)
	GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error)
	AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error
	GeneratePresignedURL(ctx context.Context, key string) (string, error)
	Config() *config.S3CompatibleDFS
}
