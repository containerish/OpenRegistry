package router

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/yor/clair/v1/server"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
)

func RegisterVulnScaningRoutes(
	userStore users.UserStore,
	clairConfig *config.ClairIntegration,
	authConfig *config.Auth,
	logger telemetry.Logger,
	layerLinkReader server.LayerLinkReader,
	prePresignedURLGenerator server.PresignedURLGenerator,
	allowedOrigins []string,
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
			vulnScanningCors := cors.New(cors.Options{
				AllowedOrigins: allowedOrigins,
				AllowOriginFunc: func(origin string) bool {
					return strings.HasSuffix(origin, "openregistry.dev") ||
						strings.HasSuffix(origin, "cntr.sh") ||
						strings.HasSuffix(origin, "openregistry-web.pages.dev") ||
						strings.Contains(origin, "localhost")
				},
				AllowedMethods: []string{
					http.MethodOptions, http.MethodGet, http.MethodPost,
				},
				AllowedHeaders: []string{
					"Origin",
					"Content-Type",
					"Authorization",
					"Connect-Protocol-Version",
					"Connect-Timeout-Ms",
					"Grpc-Timeout",
					"X-Grpc-Web",
					"X-User-Agent",
				},
				AllowCredentials: true,
				Debug:            true,
			})

			handler := h2c.NewHandler(vulnScanningCors.Handler(clairApi), &http2.Server{})
			color.Green("connectrpc Clair gRPC service running on: %s", addr)
			if err := http.ListenAndServe(addr, handler); err != nil {
				color.Red("connectrpc listen error: %s", err)
			}
		}()
	}
}
