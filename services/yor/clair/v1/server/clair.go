package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bufbuild/connect-go"
	clair_v1 "github.com/containerish/OpenRegistry/services/yor/clair/v1"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

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
	report, err := c.getVulnReport(ctx, manifestID)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	reportBz, err := io.ReadAll(report)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer report.Close()

	resp := &clair_v1.GetVulnerabilityReportResponse{}
	if err = protojson.Unmarshal(reportBz, resp); err != nil {
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

	result, err := c.submitManifest(ctx, req.Msg)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resultBz, err := io.ReadAll(result)
	if err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer result.Close()

	msg := &clair_v1.SubmitManifestToScanResponse{}
	if err = protojson.Unmarshal(resultBz, msg); err != nil {
		logEvent.Err(err).Send()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logEvent.Bool("success", true).Send()

	return connect.NewResponse(msg), nil
}

func (c *clair) getVulnReport(ctx context.Context, manifestID string) (io.ReadCloser, error) {
	uri := fmt.Sprintf("%s/matcher/api/v1/vulnerability_report/%s", c.config.ClairEndpoint, manifestID)

	req, err := c.newClairRequest(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (c *clair) submitManifest(
	ctx context.Context,
	manifest *clair_v1.SubmitManifestToScanRequest,
) (io.ReadCloser, error) {
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

	return res.Body, nil
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
