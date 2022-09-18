package dfs

import (
	"context"
	"io"

	"github.com/SkynetLabs/go-skynet/v2"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/types"
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
	CompleteMultipartUploadInput(
		ctx context.Context,
		uploadId string,
		key string,
		finalDigest string,
		completedParts []s3types.CompletedPart,
	) (string, error)
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	DownloadDir(skynetLink, dir string) error
	List(path string) ([]*types.Metadata, error)
	AddImage(ns string, mf, l map[string][]byte) (string, error)
	Metadata(skylink string) (*skynet.Metadata, error)
}
