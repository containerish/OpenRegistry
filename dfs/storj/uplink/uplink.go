package uplink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/fatih/color"
	"storj.io/uplink"
	"storj.io/uplink/edge"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

type storjUplink struct {
	client *uplink.Project
	access *uplink.Access
	config *config.Storj
	bucket string
	env    config.Environment
}

func New(env config.Environment, cfg *config.Storj) dfs.DFS {
	client, err := newUplinkClient(cfg)
	if err != nil {
		log.Fatalln(color.RedString(err.Error()))
	}

	access, err := uplink.ParseAccess(cfg.AccessGrantToken)
	if err != nil {
		log.Fatalln(color.RedString("ERR_STORJ_UPLINK_PARSE_ACCESS_GRANT_TOKEN: %w", err))
	}

	return &storjUplink{
		client: client,
		bucket: cfg.BucketName,
		access: access,
		config: cfg,
		env:    env,
	}
}

// CreateMultipartUpload implements dfs.DFS
func (u *storjUplink) CreateMultipartUpload(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	uploadOpts := &uplink.UploadOptions{}

	// set expiry in dev mode
	u.checkAndSetExpiry(uploadOpts)

	resp, err := u.client.BeginUpload(ctx, u.bucket, key, uploadOpts)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_BEGIN_UPLOAD: %w", err)
	}

	return resp.UploadID, nil
}

// UploadPart implements dfs.DFS
func (u *storjUplink) UploadPart(
	ctx context.Context,
	uploadId string,
	key string,
	digest string,
	partNumber int32,
	content io.ReadSeeker,
	contentLength int64,
) (s3types.CompletedPart, error) {
	if partNumber > config.MaxS3UploadParts {
		return s3types.CompletedPart{}, errors.New("ERR_TOO_MANY_PARTS")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute*20)
	defer cancel()

	//nolint:gosec
	resp, err := u.client.UploadPart(ctx, u.bucket, key, uploadId, uint32(partNumber))
	if err != nil {
		return s3types.CompletedPart{}, err
	}

	bz, err := io.ReadAll(content)
	if err != nil {
		return s3types.CompletedPart{}, err
	}

	_, err = resp.Write(bz)
	if err != nil {
		return s3types.CompletedPart{}, fmt.Errorf("ERR_STORJ_UPLINK_WRITE_TO_PART: %w", err)
	}

	if err = resp.SetETag([]byte(digest)); err != nil {
		return s3types.CompletedPart{}, fmt.Errorf("ERR_STORJ_UPLINK_SET_TAG: %w", err)
	}

	if err = resp.Commit(); err != nil {
		return s3types.CompletedPart{}, fmt.Errorf("ERR_STORJ_UPLINK_COMMIT_PART: %w", err)
	}

	return s3types.CompletedPart{
		ETag:       &digest,
		PartNumber: aws.Int32(int32(partNumber)),
	}, nil
}

// CompleteMultipartUpload implements dfs.DFS
func (u *storjUplink) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	key string,
	finalDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	resp, err := u.client.CommitUpload(ctx, u.bucket, key, uploadId, &uplink.CommitUploadOptions{
		CustomMetadata: map[string]string{
			"digest": finalDigest,
		},
	})
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_COMMIT_UPLOAD: %w", err)
	}

	return resp.Key, nil
}

// Download implements dfs.DFS
func (u *storjUplink) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*30)
	defer cancel()

	obj, err := u.client.DownloadObject(ctx, u.bucket, path, &uplink.DownloadOptions{})
	if err != nil {
		return nil, fmt.Errorf("ERR_STORJ_UPLINK_DOWNLOAD_OBJECT: %w", err)
	}

	return obj, nil
}

// AbortMultipartUpload implements dfs.DFS
func (u *storjUplink) AbortMultipartUpload(ctx context.Context, layerKey string, uploadId string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	if err := u.client.AbortUpload(ctx, u.bucket, layerKey, uploadId); err != nil {
		return fmt.Errorf("ERR_STORJ_UPLINK_ABORT_UPLOAD: %w", err)
	}

	return nil
}

// GetUploadProgress implements dfs.DFS
func (u *storjUplink) GetUploadProgress(identifier string, uploadID string) (*types.ObjectMetadata, error) {
	itr := u.client.ListUploadParts(context.Background(), u.bucket, identifier, uploadID, &uplink.ListUploadPartsOptions{})

	size := 0
	for itr.Next() {
		item := itr.Item()
		size += int(item.Size)
	}

	return &types.ObjectMetadata{
		ContentLength: size,
	}, nil
}

// Metadata implements dfs.DFS
func (u *storjUplink) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	identifier := types.GetLayerIdentifier(layer.ID)

	metadata, err := u.client.StatObject(context.Background(), u.bucket, identifier)
	if err != nil {
		return nil, err
	}

	return &types.ObjectMetadata{
		DFSLink:       metadata.Key,
		ContentLength: int(metadata.System.ContentLength),
	}, nil
}

// Upload implements dfs.DFS
func (u *storjUplink) Upload(ctx context.Context, namespace string, digest string, content []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*20)
	defer cancel()
	opts := &uplink.UploadOptions{}
	u.checkAndSetExpiry(opts)

	resp, err := u.client.UploadObject(ctx, u.bucket, namespace, opts)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_UPLOAD_OBJECT: %w", err)
	}

	if _, err = resp.Write(content); err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_UPLOAD_OBJECT_WRITE: %w", err)
	}

	if err = resp.Commit(); err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_UPLOAD_OBJECT_COMMIT: %w", err)
	}

	return resp.Info().Key, nil
}

// GeneratePresignedURL generates a public link (something like a presigned url) given the following:
func (u *storjUplink) GeneratePresignedURL(ctx context.Context, key string) (string, error) {
	perms := uplink.ReadOnlyPermission()
	perms.NotAfter = time.Now().Add(time.Minute * 5)

	var shareList []uplink.SharePrefix
	shareList = append(shareList, uplink.SharePrefix{Bucket: u.config.BucketName, Prefix: key})
	shareList = append(shareList, uplink.SharePrefix{Bucket: u.config.BucketName, Prefix: "layers"})

	access, err := u.access.Share(perms, shareList...)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_GENERATE_PRESIGNED_URL: %w", err)
	}

	cfg := &edge.Config{
		AuthServiceAddress: "auth.storjshare.io:7777",
		CertificatePEM:     []byte{},
		InsecureSkipVerify: false,
	}

	creds, err := cfg.RegisterAccess(ctx, access, &edge.RegisterAccessOptions{
		Public: true,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_GENERATE_PRESIGNED_URL_REGISTER_ACCESS: %w", err)
	}

	opts := &edge.ShareURLOptions{}
	link, err := edge.JoinShareURL(u.config.LinkShareService, creds.AccessKeyID, u.config.BucketName, key, opts)
	if err != nil {
		return "", fmt.Errorf("ERR_STORJ_UPLINK_GENERATE_PRESIGNED_URL_JOIN: %w", err)
	}

	return link, nil
}

// AddImage implements dfs.DFS
func (u *storjUplink) AddImage(ns string, mf map[string][]byte, l map[string][]byte) (string, error) {
	panic("unimplemented")
}

// List implements dfs.DFS
func (u *storjUplink) List(path string) ([]*types.Metadata, error) {
	panic("unimplemented")
}

// DownloadDir implements dfs.DFS
func (u *storjUplink) DownloadDir(dfsLink string, dir string) error {
	panic("unimplemented")
}

func (u *storjUplink) Config() *config.S3CompatibleDFS {
	return u.config.S3Config()
}
