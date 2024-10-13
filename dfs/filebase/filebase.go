package filebase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	oci_digest "github.com/opencontainers/go-digest"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

type filebase struct {
	client    *s3.Client
	preSigner *s3.PresignClient
	config    *config.S3CompatibleDFS
	bucket    string
	env       config.Environment
}

func New(env config.Environment, cfg *config.S3CompatibleDFS) dfs.DFS {
	client := dfs.NewS3Client(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)

	return &filebase{
		client: client,
		bucket: cfg.BucketName,
		preSigner: s3.NewPresignClient(client, func(po *s3.PresignOptions) {
			po.Expires = time.Minute * 20
		}),
		config: cfg,
		env:    env,
	}
}

func (fb *filebase) CreateMultipartUpload(layerKey string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:            &fb.bucket,
		Key:               &layerKey,
		ACL:               s3types.ObjectCannedACLPublicRead,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ContentEncoding:   aws.String("gzip"),
		StorageClass:      s3types.StorageClassStandard,
	}
	if fb.env == config.CI {
		input.Expires = aws.Time(time.Now().Add(time.Minute * 30))
	}
	upload, err := fb.client.CreateMultipartUpload(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_CREATE_MULTIPART_UPLOAD: %w", err)
	}

	return *upload.UploadId, nil
}

func (fb *filebase) UploadPart(
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

	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	partInput := &s3.UploadPartInput{
		Body:              content,
		Bucket:            &fb.bucket,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(digest),
		ContentLength:     &contentLength,
		Key:               &layerKey,
		PartNumber:        aws.Int32(partNumber),
		UploadId:          &uploadId,
	}

	resp, err := fb.client.UploadPart(ctx, partInput)
	if err != nil {
		return s3types.CompletedPart{}, fmt.Errorf("ERR_FILEBASE_UPLOAD_PART: %w", err)
	}

	return s3types.CompletedPart{
		ChecksumSHA256: &digest,
		ETag:           resp.ETag,
		PartNumber:     aws.Int32(int32(partNumber)),
	}, nil
}

func (fb *filebase) retryCompleteMultipartUpload(ctx context.Context, input *s3.CompleteMultipartUploadInput) error {
	var err error

	for i := 0; i < 3; i++ {
		_, err = fb.client.CompleteMultipartUpload(ctx, input)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 3)
	}

	return err
}

// ctx is used for handling any request cancellations.
// @param uploadId: string is the ID of the layer being uploaded
func (fb *filebase) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	digest, err := oci_digest.Parse(layerDigest)
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_DIGEST_PARSE: %w", err)
	}

	err = fb.retryCompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Key:             &layerKey,
		Bucket:          &fb.bucket,
		UploadId:        &uploadId,
		ChecksumSHA256:  aws.String(digest.Encoded()),
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_COMPLETE_MULTIPART_UPLOAD: %w", err)
	}

	resp, err := fb.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &fb.bucket,
		Key:          &layerKey,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_COMPLETE_MULTIPART_UPLOAD_HEAD: %w", err)
	}

	cid := resp.Metadata["cid"]
	if cid == "" {
		return "", fmt.Errorf("ERR_FILEBASE_CID_NOT_FOUND: %w", err)
	}

	return cid, nil
}

func (fb *filebase) Upload(ctx context.Context, namespace, digest string, content []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket:            &fb.bucket,
		Key:               &namespace,
		ACL:               s3types.ObjectCannedACLPublicRead,
		Body:              bytes.NewBuffer(content),
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    &digest,
		ContentLength:     aws.Int64(int64(len(content))),
		StorageClass:      s3types.StorageClassStandard,
	}
	if fb.env == config.CI {
		input.Expires = aws.Time(time.Now().Add(time.Minute * 30))
	}
	_, err := fb.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_UPLOAD_OBJECT: %w", err)
	}

	resp, err := fb.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &fb.bucket,
		Key:          &namespace,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_FILEBASE_UPLOAD_OBJECT_HEAD: %w", err)
	}

	cid := resp.Metadata["cid"]
	if cid == "" {
		return "", fmt.Errorf("ERR_FILEBASE_CID_NOT_FOUND: %w", err)
	}

	return cid, nil
}

func (fb *filebase) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket:       &fb.bucket,
		Key:          &path,
		ChecksumMode: s3types.ChecksumModeEnabled,
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	resp, err := fb.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ERR_FILEBASE_GET_OBJECT: %w", err)
	}

	return resp.Body, nil
}

func (fb *filebase) DownloadDir(dfsLink, dir string) error {
	return nil
}

func (fb *filebase) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (fb *filebase) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

// Metadata API returns the HEADERS for an object. This object can be a manifest or a layer.
// This API is usually a little behind when it comes to fetching the details for an uploaded object.
// This is why we put it in a retry loop and break it as soon as we get the data
func (fb *filebase) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	var resp *s3.HeadObjectOutput
	var err error

	identifier := types.GetLayerIdentifier(layer.ID)
	for i := 3; i > 0; i-- {
		resp, err = fb.client.HeadObject(context.Background(), &s3.HeadObjectInput{
			Bucket:       &fb.bucket,
			Key:          &identifier,
			ChecksumMode: s3types.ChecksumModeEnabled,
		})
		if err != nil {
			// cool off
			time.Sleep(time.Second * 3)
			continue
		}

		break
	}
	if err != nil {
		return nil, fmt.Errorf("ERR_FILEBASE_METADATA_HEAD: %w", err)
	}

	cid := resp.Metadata["cid"]
	if cid == "" {
		cid = identifier
	}

	return &types.ObjectMetadata{
		ContentType:   *resp.ContentType,
		Etag:          *resp.ETag,
		DFSLink:       cid,
		ContentLength: int(*resp.ContentLength),
	}, nil
}

func (fb *filebase) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	partsResp, err := fb.client.ListParts(context.Background(), &s3.ListPartsInput{
		Bucket:   &fb.bucket,
		Key:      &identifier,
		UploadId: &uploadID,
	})
	if err != nil {
		return nil, fmt.Errorf("ERR_FILEBASE_UPLOAD_PROGRESS: %w", err)
	}

	var uploadedSize int64
	for _, p := range partsResp.Parts {
		uploadedSize += *p.Size
	}

	return &types.ObjectMetadata{
		ContentLength: int(uploadedSize),
	}, nil
}

func (fb *filebase) AbortMultipartUpload(ctx context.Context, layerKey, uploadId string) error {
	_, err := fb.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &fb.bucket,
		Key:      &layerKey,
		UploadId: &uploadId,
	})
	if err != nil {
		return fmt.Errorf("FB_ABORT_MULTI_PART_UPLOAD: %w", err)
	}

	return nil
}

func (fb *filebase) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	// Filebase+IPFS content can be directly resolved over an IPFS gateway
	return fmt.Sprintf("%s/%s", fb.config.DFSLinkResolver, key), nil
}

func (fb *filebase) Config() *config.S3CompatibleDFS {
	return fb.config
}
