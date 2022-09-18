package filebase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SkynetLabs/go-skynet/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/types"
	"github.com/opencontainers/go-digest"
)

type filebase struct {
	client *s3.Client
	bucket string
}

func New(cfg *config.S3CompatibleDFS) dfs.DFS {
	client := dfs.NewS3Client(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	return &filebase{
		client: client,
		bucket: cfg.BucketName,
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
	upload, err := fb.client.CreateMultipartUpload(context.Background(), input)
	if err != nil {
		return "", err
	}

	return *upload.UploadId, nil
}

func (fb *filebase) UploadPart(
	ctx context.Context,
	uploadId string,
	layerKey string,
	digest string,
	partNumber int64,
	content io.ReadSeeker,
	contentLength int64,
) (s3types.CompletedPart, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*20)
	defer cancel()

	partInput := &s3.UploadPartInput{
		Body:              content,
		Bucket:            &fb.bucket,
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(digest),
		ContentLength:     contentLength,
		Key:               &layerKey,
		PartNumber:        int32(partNumber),
		UploadId:          &uploadId,
	}

	resp, err := fb.client.UploadPart(ctx, partInput)
	if err != nil {
		return s3types.CompletedPart{}, err
	}
	return s3types.CompletedPart{
		ChecksumSHA256: &digest,
		ETag:           resp.ETag,
		PartNumber:     int32(partNumber),
	}, nil

}

// ctx is used for handling any request cancellations.
// @param uploadId: string is the ID of the layer being uploaded
func (fb *filebase) CompleteMultipartUploadInput(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	dig, err := digest.Parse(layerDigest)
	if err != nil {
		return "", fmt.Errorf("ERR_HEX_DECODE: %w", err)
	}

	_, err = fb.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Key:             &layerKey,
		Bucket:          &fb.bucket,
		UploadId:        &uploadId,
		ChecksumSHA256:  aws.String(dig.Encoded()),
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: completedParts},
	})
	if err != nil {
		return "", fmt.Errorf("ERR_COMPLETE_UPLOAD: %w", err)
	}

	// fb.client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
	// 	Bucket: &fb.bucket,
	// 	Key:    aws.String("layers/" + layerKey),
	// 	Tagging: &s3types.Tagging{
	// 		TagSet: []s3types.Tag{
	// 			{Key: aws.String("digest"), Value: aws.String(finalDigest)},
	// 		},
	// 	},
	// })
	resp, err := fb.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &fb.bucket,
		Key:          &layerKey,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", err
	}

	return resp.Metadata["cid"], nil
}

func (fb *filebase) Upload(ctx context.Context, namespace, digest string, content []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*20)
	defer cancel()

	_, err := fb.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            &fb.bucket,
		Key:               &namespace,
		ACL:               s3types.ObjectCannedACLPublicRead,
		Body:              bytes.NewBuffer(content),
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    &digest,
		ContentLength:     int64(len(content)),
		StorageClass:      s3types.StorageClassStandard,
	})
	if err != nil {
		return "", fmt.Errorf("ERR_PUT_OBJECT: %w", err)
	}

	resp, err := fb.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       &fb.bucket,
		Key:          &namespace,
		ChecksumMode: s3types.ChecksumModeEnabled,
	})
	if err != nil {
		return "", err
	}

	return resp.Metadata["cid"], nil
}

func (fb *filebase) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket:       &fb.bucket,
		Key:          &path,
		ChecksumMode: s3types.ChecksumModeEnabled,
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*20)
	defer cancel()

	resp, err := fb.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_OBJECT: %w", err)
	}

	return resp.Body, nil
}
func (fb *filebase) DownloadDir(skynetLink, dir string) error {
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
func (fb *filebase) Metadata(identifier string) (*skynet.Metadata, error) {
	var resp *s3.HeadObjectOutput
	var err error

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
		return nil, err
	}

	return &skynet.Metadata{
		ContentType:   *resp.ContentType,
		Etag:          *resp.ETag,
		Skylink:       resp.Metadata["cid"],
		ContentLength: int(resp.ContentLength),
	}, nil
}

//func (fb *filebase) buildCopySourceRange(start int64, objectSize int64) string {
//	end := start + max_part_size - 1
//	if end > objectSize {
//		end = objectSize - 1
//	}
//	startRange := strconv.FormatInt(start, 10)
//	stopRange := strconv.FormatInt(end, 10)
//	return "bytes=" + startRange + "-" + stopRange
//}

////function that starts, perform each part upload, and completes the copy
//func (fb *filebase) MultiPartCopy(svc *s3.Client, fileSize int64, sourceKey string, destKey string) error {
//	ctx, cancelFn := context.WithTimeout(context.TODO(), 10*time.Minute)
//	defer cancelFn()

//	//struct for starting a multipart upload
//	startInput := s3.CreateMultipartUploadInput{
//		Bucket: &destBucket,
//		Key:    &destKey,
//	}

//	//send command to start copy and get the upload id as it is needed later
//	var uploadId string
//	createOutput, err := svc.CreateMultipartUpload(ctx, &startInput)
//	if err != nil {
//		return err
//	}
//	if createOutput != nil {
//		if createOutput.UploadId != nil {
//			uploadId = *createOutput.UploadId
//		}
//	}
//	if uploadId == "" {
//		return errors.New("No upload id found in start upload request")
//	}

//	var i int64
//	var partNumber int32 = 1
//	copySource := "/" + fb.bucket + "/" + sourceKey

//	parts := make([]types.CompletedPart, 0)

//	// for i = 0; i < fileSize; i += max_part_size {
//	copyRange := fb.buildCopySourceRange(i, fileSize)
//	partInput := &s3.UploadPartInput{
//		Bucket:               &fb.bucket,
//		Key:                  &destKey,
//		PartNumber:           partNumber,
//		UploadId:             &uploadId,
//		Body:                 nil,
//		ChecksumAlgorithm:    "",
//		ChecksumCRC32:        new(string),
//		ChecksumCRC32C:       new(string),
//		ChecksumSHA1:         new(string),
//		ChecksumSHA256:       new(string),
//		ContentLength:        0,
//		ContentMD5:           new(string),
//		ExpectedBucketOwner:  new(string),
//		RequestPayer:         "",
//		SSECustomerAlgorithm: new(string),
//		SSECustomerKey:       new(string),
//		SSECustomerKeyMD5:    new(string),
//	}
//	fb.client.UploadPart(ctx, partInput)
//	partResp, err := svc.UploadPartCopy(context.TODO(), &partInput)

//	if err != nil {
//		abortIn := s3.AbortMultipartUploadInput{
//			UploadId: &uploadId,
//		}
//		//ignoring any errors with aborting the copy
//		svc.AbortMultipartUpload(context.TODO(), &abortIn)
//		return fmt.Errorf("Error uploading part %d : %w", partNumber, err)
//	}

//	//copy etag and part number from response as it is needed for completion
//	if partResp != nil {
//		partNum := partNumber
//		etag := strings.Trim(*partResp.CopyPartResult.ETag, "\"")
//		cPart := types.CompletedPart{
//			ETag:       &etag,
//			PartNumber: partNum,
//		}
//		parts = append(parts, cPart)
//	}
//	partNumber++
//	if partNumber%50 == 0 {
//	}
//	// }

//	//create struct for completing the upload
//	mpu := types.CompletedMultipartUpload{
//		Parts: parts,
//	}

//	//complete actual upload
//	//does not actually copy if the complete command is not received
//	complete := s3.CompleteMultipartUploadInput{
//		Bucket:          &destBucket,
//		Key:             &destKey,
//		UploadId:        &uploadId,
//		MultipartUpload: &mpu,
//	}
//	compOutput, err := svc.CompleteMultipartUpload(context.TODO(), &complete)
//	if err != nil {
//		return fmt.Errorf("Error completing upload: %w", err)
//	}
//	if compOutput != nil {
//	}
//	return nil
//}
