// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: services/kon/github_actions/v1/events.proto

package github_actions_v1connect

import (
	context "context"
	errors "errors"
	connect_go "github.com/bufbuild/connect-go"
	_ "github.com/containerish/OpenRegistry/services/kon/github_actions/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect_go.IsAtLeastVersion0_1_0

const (
	// GitHubWebhookListenerServiceName is the fully-qualified name of the GitHubWebhookListenerService
	// service.
	GitHubWebhookListenerServiceName = "services.kon.github_actions.v1.GitHubWebhookListenerService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// GitHubWebhookListenerServiceListenProcedure is the fully-qualified name of the
	// GitHubWebhookListenerService's Listen RPC.
	GitHubWebhookListenerServiceListenProcedure = "/services.kon.github_actions.v1.GitHubWebhookListenerService/Listen"
)

// GitHubWebhookListenerServiceClient is a client for the
// services.kon.github_actions.v1.GitHubWebhookListenerService service.
type GitHubWebhookListenerServiceClient interface {
	Listen(context.Context, *connect_go.Request[structpb.Struct]) (*connect_go.Response[emptypb.Empty], error)
}

// NewGitHubWebhookListenerServiceClient constructs a client for the
// services.kon.github_actions.v1.GitHubWebhookListenerService service. By default, it uses the
// Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewGitHubWebhookListenerServiceClient(httpClient connect_go.HTTPClient, baseURL string, opts ...connect_go.ClientOption) GitHubWebhookListenerServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &gitHubWebhookListenerServiceClient{
		listen: connect_go.NewClient[structpb.Struct, emptypb.Empty](
			httpClient,
			baseURL+GitHubWebhookListenerServiceListenProcedure,
			opts...,
		),
	}
}

// gitHubWebhookListenerServiceClient implements GitHubWebhookListenerServiceClient.
type gitHubWebhookListenerServiceClient struct {
	listen *connect_go.Client[structpb.Struct, emptypb.Empty]
}

// Listen calls services.kon.github_actions.v1.GitHubWebhookListenerService.Listen.
func (c *gitHubWebhookListenerServiceClient) Listen(ctx context.Context, req *connect_go.Request[structpb.Struct]) (*connect_go.Response[emptypb.Empty], error) {
	return c.listen.CallUnary(ctx, req)
}

// GitHubWebhookListenerServiceHandler is an implementation of the
// services.kon.github_actions.v1.GitHubWebhookListenerService service.
type GitHubWebhookListenerServiceHandler interface {
	Listen(context.Context, *connect_go.Request[structpb.Struct]) (*connect_go.Response[emptypb.Empty], error)
}

// NewGitHubWebhookListenerServiceHandler builds an HTTP handler from the service implementation. It
// returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewGitHubWebhookListenerServiceHandler(svc GitHubWebhookListenerServiceHandler, opts ...connect_go.HandlerOption) (string, http.Handler) {
	mux := http.NewServeMux()
	mux.Handle(GitHubWebhookListenerServiceListenProcedure, connect_go.NewUnaryHandler(
		GitHubWebhookListenerServiceListenProcedure,
		svc.Listen,
		opts...,
	))
	return "/services.kon.github_actions.v1.GitHubWebhookListenerService/", mux
}

// UnimplementedGitHubWebhookListenerServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedGitHubWebhookListenerServiceHandler struct{}

func (UnimplementedGitHubWebhookListenerServiceHandler) Listen(context.Context, *connect_go.Request[structpb.Struct]) (*connect_go.Response[emptypb.Empty], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("services.kon.github_actions.v1.GitHubWebhookListenerService.Listen is not implemented"))
}