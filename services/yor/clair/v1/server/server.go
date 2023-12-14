package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/config"
	connect_v1 "github.com/containerish/OpenRegistry/services/yor/clair/v1/clairconnect"
	"github.com/containerish/OpenRegistry/telemetry"
)

type (
	clair struct {
		http   *http.Client
		logger telemetry.Logger
		config *config.ClairIntegration
		mu     *sync.RWMutex
	}
)

func NewClairClient(
	config *config.ClairIntegration,
	logger telemetry.Logger,
) *http.ServeMux {
	if !config.Enabled {
		return nil
	}

	httpClient := &http.Client{
		Timeout:   time.Minute * 3,
		Transport: http.DefaultTransport,
	}

	server := &clair{
		logger: logger,
		config: config,
		mu:     &sync.RWMutex{},
		http:   httpClient,
	}

	// interceptors := connect.WithInterceptors(NewGithubAppInterceptor(logger, ghStore, nil, authConfig))
	mux := http.NewServeMux()
	mux.Handle(connect_v1.NewClairServiceHandler(server))
	return mux
}
