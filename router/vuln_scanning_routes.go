package router

import (
	"fmt"
	"net"
	"net/http"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/yor/clair/v1/server"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func RegisterVulnScaningRoutes(
	userStore users.UserStore,
	clairConfig *config.ClairIntegration,
	authConfig *config.Auth,
	logger telemetry.Logger,
	layerLinkReader server.LayerLinkReader,
	prePresignedURLGenerator server.PresignedURLGenerator,
) {
	if clairConfig != nil && clairConfig.Enabled {
		clairApi := server.NewClairClient(
			userStore,
			clairConfig,
			authConfig,
			logger,
			layerLinkReader,
			prePresignedURLGenerator,
		)
		go func() {
			addr := net.JoinHostPort(clairConfig.Host, fmt.Sprintf("%d", clairConfig.Port))
			color.Green("connect-go Clair gRPC service running on: %s", addr)
			if err := http.ListenAndServe(addr, h2c.NewHandler(clairApi, &http2.Server{})); err != nil {
				color.Red("connect-go listen error: %s", err)
			}
		}()
	}
}
