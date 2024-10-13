package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/encoding/protojson"

	clair_v1 "github.com/containerish/OpenRegistry/services/yor/clair/v1"
)

func (c *clair) EnableVulnerabilityScanning(
	ctx context.Context,
	req *connect.Request[clair_v1.EnableVulnerabilityScanningRequest],
) (
	*connect.Response[clair_v1.EnableVulnerabilityScanningResponse],
	error,
) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("UNIMPLEMENTED"))
}

func (c *clair) GetVulnerabilityReport(
	ctx context.Context,
	req *connect.Request[clair_v1.GetVulnerabilityReportRequest],
) (
	*connect.Response[clair_v1.GetVulnerabilityReportResponse],
	error,
) {
	logEvent := c.logger.Debug().Str("method", "GetVulnerabilityReport")

	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	manifestID := req.Msg.GetManifestId()
	logEvent.Str("manifest", manifestID)
	reportBz, err := c.getVulnReport(ctx, manifestID)
	if err != nil {
		var errMap map[string]any
		_ = json.Unmarshal(reportBz, &errMap)
		logEvent.Err(err).Any("get_manifest_err", errMap).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &clair_v1.GetVulnerabilityReportResponse{}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(reportBz, resp); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()
	return connect.NewResponse(resp), nil
}

// SubmitManifestToScan implements clairconnect.ClairServiceHandler.
func (c *clair) SubmitManifestToScan(
	ctx context.Context,
	req *connect.Request[clair_v1.SubmitManifestToScanRequest],
) (
	*connect.Response[clair_v1.SubmitManifestToScanResponse],
	error,
) {
	logEvent := c.logger.Debug().Str("method", "SubmitManifestToScan")

	err := req.Msg.Validate()
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	logEvent.Str("manifest", req.Msg.GetHash())

	dfsLinks, err := c.layerLinkReader(ctx, req.Msg.GetHash())
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	layers := make([]*clair_v1.ClairDescriptor, len(dfsLinks))
	for i, link := range dfsLinks {
		presignedURL, signErr := c.prePresignedURLGenerator(ctx, link.DFSLink)
		if signErr != nil {
			logEvent.Err(err).Send()
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		layers[i] = &clair_v1.ClairDescriptor{
			Hash: link.Digest,
			Uri:  presignedURL,
		}
	}

	body := &clair_v1.ClairIndexManifestRequest{
		Hash:   req.Msg.GetHash(),
		Layers: layers,
	}

	resultBz, err := c.submitManifest(ctx, body)
	if err != nil {
		var errMap map[string]any
		_ = json.Unmarshal(resultBz, &errMap)
		logEvent.Err(err).Any("manifest_submit_err", errMap).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	msg := &clair_v1.SubmitManifestToScanResponse{}
	if err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(resultBz, msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()

	return connect.NewResponse(msg), nil
}

func (c *clair) getVulnReport(ctx context.Context, manifestID string) ([]byte, error) {
	uri := fmt.Sprintf("%s/matcher/api/v1/vulnerability_report/%s", c.config.ClairEndpoint, manifestID)

	req, err := c.newClairRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	bz, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ERR_GET_VULN_REPORT: READ_RESPONSE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return bz, nil
	}

	return bz, fmt.Errorf("ERR_GET_VULN_REPORT: INVALID_RESPONSE: %d", resp.StatusCode)
}

func (c *clair) submitManifest(
	ctx context.Context,
	manifest *clair_v1.ClairIndexManifestRequest,
) ([]byte, error) {
	uri := fmt.Sprintf("%s/indexer/api/v1/index_report", c.config.ClairEndpoint)

	bz, err := protojson.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	req, err := c.newClairRequest(ctx, http.MethodPost, uri, bytes.NewBuffer(bz))
	if err != nil {
		return nil, err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	bz, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("ERR_SUBMIT_MANIFEST_TO_SCAN: READ_RESPONSE: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 200 && res.StatusCode <= 300 {
		return bz, nil
	}

	return bz, fmt.Errorf("ERR_SUBMIT_MANIFEST_TO_SCAN: CODE: %d", res.StatusCode)
}

func (c *clair) newClairRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("ERR_NEW_CLAIR_REQ: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "quay",
	})

	authToken, err := token.SignedString([]byte(c.config.AuthToken))
	if err != nil {
		return nil, fmt.Errorf("ERR_NEW_CLAIR_REQ: SignAuthToken: %w - AuthToken: %s", err, c.config.AuthToken)
	}

	req.Header.Set("Authorization", "Bearer "+authToken)

	return req, nil
}
