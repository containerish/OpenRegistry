package storj

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

type storj struct {
	client    *s3.Client
	preSigner *s3.PresignClient
	config    *config.S3CompatibleDFS
	bucket    string
	env       config.Environment
}

func New(env config.Environment, cfg *config.S3CompatibleDFS) dfs.DFS {
	client := dfs.NewS3Client(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)

	return &storj{
		client:    client,
		bucket:    cfg.BucketName,
		preSigner: s3.NewPresignClient(client),
		config:    cfg,
		env:       env,
	}
}

func (sj *storj) CreateMultipartUpload(layerKey string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:            &sj.bucket,
		Key:               &layerKey,
		ACL:               s3types.ObjectCannedACLPublicRead,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ContentEncoding:   aws.String("gzip"),
		StorageClass:      s3types.StorageClassStandard,
	}
	if sj.env == config.CI {
		input.Expires = aws.Time(time.Now().Add(time.Minute * 30))
	}
	upload, err := sj.client.CreateMultipartUpload(context.Background(), input)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_CREATE_MULTIPART_UPLOAD: %w", err)
	}

	return *upload.UploadId, nil
}

func (sj *storj) UploadPart(
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
		Bucket:            &sj.bucket,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(digest),
		ContentLength:     &contentLength,
		Key:               &layerKey,
		PartNumber:        aws.Int32(partNumber),
		UploadId:          &uploadId,
	}

	resp, err := sj.client.UploadPart(ctx, partInput)
	if err != nil {
		return s3types.CompletedPart{}, fmt.Errorf("ERR_STORJ_UPLOAD_PART: %w", err)
	}

	return s3types.CompletedPart{
		ChecksumSHA256: &digest,
		ETag:           resp.ETag,
		PartNumber:     aws.Int32(int32(partNumber)),
	}, nil
}

// ctx is used for handling any request cancellations.
// @param uploadId: string is the ID of the layer being uploaded
func (sj *storj) CompleteMultipartUpload(
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
		return "", fmt.Errorf("ERR_STORJ_DIGEST_PARSE: %w", err)
	}

	_, err = sj.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Key:             &layerKey,
		Bucket:          &sj.bucket,
		UploadId:        &uploadId,
		ChecksumSHA256:  aws.String(digest.Encoded()),
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_COMPLETE_MULTIPART_UPLOAD: %w", err)
	}

	_, err = sj.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &sj.bucket,
		Key:          &layerKey,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_COMPLETE_MULTIPART_UPLOAD_HEAD: %w", err)
	}

	return layerKey, nil
}

func (sj *storj) Upload(ctx context.Context, identifier, digest string, content []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	input := &s3.PutObjectInput{
		Bucket:            &sj.bucket,
		Key:               &identifier,
		ACL:               s3types.ObjectCannedACLPublicRead,
		Body:              bytes.NewBuffer(content),
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    &digest,
		ContentLength:     aws.Int64(int64(len(content))),
		StorageClass:      s3types.StorageClassStandard,
	}
	if sj.env == config.CI {
		input.Expires = aws.Time(time.Now().Add(time.Minute * 30))
	}

	_, err := sj.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLOAD_OBJECT: %w", err)
	}

	_, err = sj.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &sj.bucket,
		Key:          &identifier,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLOAD_OBJECT_HEAD: %w", err)
	}

	return identifier, nil
}

// Download method returns an io.ReadCloser. The end user/consumer is responsible to close the io.ReadCloser
func (sj *storj) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket:       &sj.bucket,
		Key:          &path,
		ChecksumMode: s3types.ChecksumModeEnabled,
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	resp, err := sj.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ERR_STORJ_GET_OBJECT: %w", err)
	}

	return resp.Body, nil
}

func (sj *storj) DownloadDir(dfsLink, dir string) error {
	return nil
}

func (sj *storj) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (sj *storj) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

// Metadata API returns the HEADERS for an object. This object can be a manifest or a layer.
// This API is usually a little behind when it comes to fetching the details for an uploaded object.
// This is why we put it in a retry loop and break it as soon as we get the data
func (sj *storj) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	var resp *s3.HeadObjectOutput
	var err error
	id := types.GetLayerIdentifier(layer.ID)
	for i := 3; i > 0; i-- {
		resp, err = sj.client.HeadObject(context.Background(), &s3.HeadObjectInput{
			Bucket:       &sj.bucket,
			Key:          &id,
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
		return nil, fmt.Errorf("ERR_STORJ_METADATA_HEAD: %w", err)
	}

	return &types.ObjectMetadata{
		ContentType:   *resp.ContentType,
		Etag:          *resp.ETag,
		DFSLink:       id,
		ContentLength: int(*resp.ContentLength),
	}, nil
}

func (sj *storj) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	partsResp, err := sj.client.ListParts(context.Background(), &s3.ListPartsInput{
		Bucket:   &sj.bucket,
		Key:      &identifier,
		UploadId: &uploadID,
	})
	if err != nil {
		return nil, fmt.Errorf("ERR_STORJ_UPLOAD_PROGRESS: %w", err)
	}

	var uploadedSize int64
	for _, p := range partsResp.Parts {
		uploadedSize += *p.Size
	}

	return &types.ObjectMetadata{
		ContentLength: int(uploadedSize),
	}, nil
}

func (sj *storj) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	opts := &s3.GetObjectInput{
		Bucket: &sj.bucket,
		Key:    &key,
	}

	duration := func(o *s3.PresignOptions) {
		o.Expires = time.Minute * 20
	}

	resp, err := sj.preSigner.PresignGetObject(ctx, opts, duration)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_GENERATE_PRESIGNED_URL: %w", err)
	}

	return resp.URL, nil
}

func (sj *storj) AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error {
	_, err := sj.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &sj.bucket,
		Key:      &layerKey,
		UploadId: &uploadId,
	})
	if err != nil {
		return fmt.Errorf("ERR_STORJ_ABORT_MULTI_PART_UPLOAD: %w", err)
	}

	return nil
}

func (sj *storj) Config() *config.S3CompatibleDFS {
	return sj.config
}
