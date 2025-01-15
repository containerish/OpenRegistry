package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/containerish/OpenRegistry/config"
	connect_v1 "github.com/containerish/OpenRegistry/services/yor/clair/v1/clairconnect"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
)

type (
	clair struct {
		http                     *http.Client
		logger                   telemetry.Logger
		config                   *config.ClairIntegration
		userGetter               users.UserStore
		authConfig               *config.Auth
		mu                       *sync.RWMutex
		layerLinkReader          LayerLinkReader
		prePresignedURLGenerator PresignedURLGenerator
	}

	LayerLinkReader       func(ctx context.Context, manifestDigest string) ([]*types.ContainerImageLayer, error)
	PresignedURLGenerator func(ctx context.Context, path string) (string, error)
)

func NewClairClient(
	userStore users.UserStore,
	config *config.ClairIntegration,
	authConfig *config.Auth,
	logger telemetry.Logger,
	layerLinkReader LayerLinkReader,
	prePresignedURLGenerator PresignedURLGenerator,
) *http.ServeMux {
	if !config.Enabled {
		return nil
	}

	httpClient := &http.Client{
		Timeout:   time.Minute * 3,
		Transport: http.DefaultTransport,
	}

	server := &clair{
		logger:                   logger,
		config:                   config,
		mu:                       &sync.RWMutex{},
		http:                     httpClient,
		userGetter:               userStore,
		authConfig:               authConfig,
		layerLinkReader:          layerLinkReader,
		prePresignedURLGenerator: prePresignedURLGenerator,
	}

	interceptors := connect.WithInterceptors(server.NewJWTInterceptor())
	mux := http.NewServeMux()
	mux.Handle(connect_v1.NewClairServiceHandler(server, interceptors))
	return mux
}
