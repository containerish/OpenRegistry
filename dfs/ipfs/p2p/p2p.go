package p2p

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/fatih/color"
	"github.com/google/uuid"
	boxo_files "github.com/ipfs/boxo/files"
	boxo_path "github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/multiformats/go-multiaddr"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/dfs"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

const (
	multipartSessionTimeout    = time.Minute * 5
	multipartPartUploadTimeout = time.Minute * 5
)

type (
	ipfsP2p struct {
		node          *rpc.HttpApi
		config        *config.IpfsDFS
		uploadSession *haxmap.Map[string, *multipartSession]
		uploadParts   *haxmap.Map[string, *uploadParts]
	}

	multipartSession struct {
		expiresAt time.Time
		id        string
	}

	uploadParts struct {
		buf       *bytes.Buffer
		expiresAt time.Time
	}
)

func New(config *config.IpfsDFS) dfs.DFS {
	var node *rpc.HttpApi

	if config.Local {
		localNode, err := rpc.NewLocalApi()
		if err != nil {
			log.Fatalln(color.RedString(err.Error()))
		}

		baseDir := os.Getenv(rpc.EnvDir)
		if baseDir == "" {
			baseDir = rpc.DefaultPathRoot
		}

		localAddr, err := rpc.ApiAddr(baseDir)
		if err != nil {
			log.Fatalln(color.RedString("error parsing local ipfs node config: %w", err))
		}

		config.RPCMultiAddr = localAddr.String()

		node = localNode
	} else {
		addr, err := multiaddr.NewMultiaddr(config.RPCMultiAddr)
		if err != nil {
			log.Fatalln(color.RedString(err.Error()))
		}

		remoteNode, err := rpc.NewApi(addr)
		if err != nil {
			log.Fatalln(color.RedString(err.Error()))
		}

		node = remoteNode
	}

	dfs := &ipfsP2p{
		node:          node,
		config:        config,
		uploadSession: haxmap.New[string, *multipartSession](),
		uploadParts:   haxmap.New[string, *uploadParts](),
	}

	// run garbage collection in background
	go dfs.gc()
	return dfs
}

func (ipfs *ipfsP2p) CreateMultipartUpload(layerKey string) (string, error) {
	sessionId := uuid.New()

	ipfs.uploadSession.Set(sessionId.String(), &multipartSession{
		id:        layerKey,
		expiresAt: time.Now().Add(multipartSessionTimeout),
	})

	return sessionId.String(), nil
}

func (ipfs *ipfsP2p) UploadPart(
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

	session, ok := ipfs.uploadSession.Get(uploadId)
	if !ok {
		return s3types.CompletedPart{}, fmt.Errorf("UploadPart: multipart session not found")
	}

	if session.id != layerKey {
		return s3types.CompletedPart{}, fmt.Errorf("UploadPart: invalid layer key: %s", layerKey)
	}

	partBz, err := io.ReadAll(content)
	if err != nil {
		return s3types.CompletedPart{}, err
	}

	part, ok := ipfs.uploadParts.Get(uploadId)
	if ok {
		writtenBz, err := part.buf.Write(partBz)
		if err != nil {
			return s3types.CompletedPart{}, err
		}

		if writtenBz != int(contentLength) {
			return s3types.CompletedPart{}, fmt.Errorf("UploadPart: upload part content size mismatch")
		}
		ipfs.uploadParts.Set(uploadId, part)
	} else {
		buf := bytes.NewBuffer(partBz)
		ipfs.uploadParts.Set(uploadId, &uploadParts{
			buf:       buf,
			expiresAt: time.Now().Add(multipartSessionTimeout),
		})
	}

	return s3types.CompletedPart{
		ChecksumSHA256: &digest,
		PartNumber:     aws.Int32(partNumber),
	}, nil
}

// ctx is used for handling any request cancellations.
// @param uploadId: string is the ID of the layer being uploaded
func (ipfs *ipfsP2p) CompleteMultipartUpload(
	ctx context.Context,
	uploadId string,
	layerKey string,
	layerDigest string,
	completedParts []s3types.CompletedPart,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	session, ok := ipfs.uploadSession.Get(uploadId)
	if !ok {
		return "", fmt.Errorf("UploadPart: multipart session not found")
	}
	if layerKey != session.id {
		return "", fmt.Errorf("UploadPart: invalid layer key: %s", layerKey)
	}

	if parts, ok := ipfs.uploadParts.Get(uploadId); ok {
		fd := boxo_files.NewBytesFile(parts.buf.Bytes())
		path, err := ipfs.node.Unixfs().Add(ctx, fd)
		if err != nil {
			return "", fmt.Errorf("P2P: CompleteMultipartUpload: Put Object Error: %w", err)
		}

		if ipfs.config.Pinning {
			if err = ipfs.node.Pin().Add(ctx, path); err != nil {
				return "", err
			}
		}

		// cleanup session & cached layer parts
		ipfs.uploadSession.Del(uploadId)
		ipfs.uploadParts.Del(uploadId)

		return path.RootCid().String(), nil
	}

	return "", fmt.Errorf("CompleteMultipartUpload: upload session not found")
}

func (ipfs *ipfsP2p) Upload(ctx context.Context, namespace, digest string, content []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	fd := boxo_files.NewBytesFile(content)
	path, err := ipfs.node.Unixfs().Add(ctx, fd)
	if err != nil {
		return "", err
	}

	if ipfs.config.Pinning {
		if err = ipfs.node.Pin().Add(ctx, path); err != nil {
			return "", err
		}
	}

	return path.RootCid().String(), nil
}

func (ipfs *ipfsP2p) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ipfsPath, err := boxo_path.NewPath(path)
	if err != nil {
		return nil, err
	}

	resolvedPath, _, err := ipfs.node.ResolvePath(ctx, ipfsPath)
	if err != nil {
		return nil, err
	}

	node, err := ipfs.node.Dag().Get(ctx, resolvedPath.RootCid())
	// node, err := ipfs.node.Object().Get(ctx, ipfsPath)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(node.RawData())
	return io.NopCloser(buf), nil
}

func (ipfs *ipfsP2p) DownloadDir(dfsLink, dir string) error {
	return nil
}

func (ipfs *ipfsP2p) List(path string) ([]*types.Metadata, error) {
	return nil, nil
}

func (ipfs *ipfsP2p) AddImage(ns string, mf, l map[string][]byte) (string, error) {
	return "", nil
}

// Metadata API returns the HEADERS for an object. This object can be a manifest or a layer.
// This API is usually a little behind when it comes to fetching the details for an uploaded object.
// This is why we put it in a retry loop and break it as soon as we get the data
func (ipfs *ipfsP2p) Metadata(layer *types.ContainerImageLayer) (*types.ObjectMetadata, error) {
	identifier := "/ipfs/" + layer.DFSLink
	idParts := strings.Split(identifier, "/")
	if len(idParts) > 1 {
		identifier = "/ipfs/" + idParts[len(idParts)-1]
	}

	ipfsPath, err := boxo_path.NewPath(identifier)
	if err != nil {
		return nil, err
	}

	stat, err := ipfs.node.Block().Stat(context.TODO(), ipfsPath)
	if err != nil {
		return nil, err
	}

	return &types.ObjectMetadata{
		DFSLink:       ipfsPath.String(),
		ContentLength: stat.Size(),
	}, nil
}

func (ipfs *ipfsP2p) GetUploadProgress(identifier, uploadID string) (*types.ObjectMetadata, error) {
	uploadedSize := 0
	if partsResp, ok := ipfs.uploadParts.Get(uploadID); ok {
		uploadedSize = partsResp.buf.Len()
	}

	return &types.ObjectMetadata{
		ContentLength: int(uploadedSize),
	}, nil
}

func (ipfs *ipfsP2p) AbortMultipartUpload(ctx context.Context, layerKey, uploadId string) error {
	ipfs.uploadSession.Del(uploadId)
	ipfs.uploadParts.Del(uploadId)
	return nil
}

func (ipfs *ipfsP2p) GeneratePresignedURL(ctx context.Context, cid string) (string, error) {
	cidParts := strings.Split(cid, "/")
	if len(cidParts) == 1 {
		cid = "ipfs/" + cid
	}

	return fmt.Sprintf("%s/%s", ipfs.config.GatewayEndpoint, cid), nil
}

// garbage collection
func (ipfs *ipfsP2p) gc() {
	ticker := time.NewTicker(time.Second * 3)
	for now := range ticker.C {
		var deleteParts []string
		var deleteSessions []string
		ipfs.uploadParts.ForEach(func(key string, value *uploadParts) bool {
			if now.Unix() > value.expiresAt.Unix() {
				deleteParts = append(deleteParts, key)
			}
			return true
		})
		ipfs.uploadSession.ForEach(func(key string, value *multipartSession) bool {
			if now.Unix() > value.expiresAt.Unix() {
				deleteSessions = append(deleteSessions, key)
			}
			return true
		})

		if len(deleteParts) > 0 {
			ipfs.uploadParts.Del(deleteParts...)
		}
		if len(deleteSessions) > 0 {
			ipfs.uploadSession.Del(deleteSessions...)
		}

		ticker.Reset(time.Second * 3)
	}
}

func (ipfs *ipfsP2p) Config() *config.S3CompatibleDFS {
	return &config.S3CompatibleDFS{}
}
