package router

import (
	"fmt"
	"net"
	"net/http"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/yor/clair/v1/server"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func RegisterVulnScaningRoutes(
	config *config.ClairIntegration,
	logger telemetry.Logger,
) {
	if config != nil && config.Enabled {
		clairApi := server.NewClairClient(config, logger)
		go func() {
			addr := net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))
			color.Green("connect-go Clair gRPC service running on: %s", addr)
			if err := http.ListenAndServe(addr, h2c.NewHandler(clairApi, &http2.Server{})); err != nil {
				color.Red("connect-go listen error: %s", err)
			}
		}()
	}
}
