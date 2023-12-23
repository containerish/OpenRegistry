package otel

import (
	"log"
	"os"

	"github.com/containerish/OpenRegistry/config"
	"github.com/fatih/color"
	"github.com/honeycombio/honeycomb-opentelemetry-go"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func ConfigureOtel(config config.Honeycomb, service string, e *echo.Echo) func() {
	if config.Enabled {
		checkAndLoadHoneycombConfig(config)

		color.Green("OpenTelemetry with Honeycomb.io: Enabled")
		bsp := honeycomb.NewBaggageSpanProcessor()

		otelShutdown, err := otelconfig.ConfigureOpenTelemetry(
			otelconfig.WithServiceName(service),
			otelconfig.WithSpanProcessor(bsp),
			otelconfig.WithTracesEnabled(true),
		)
		if err != nil {
			log.Fatalln(color.RedString("ERR_CONFIGURE_OTEL: %s", err))
		}

		e.Use(otelecho.Middleware(service))
		return otelShutdown
	}

	return nil
}

func checkAndLoadHoneycombConfig(config config.Honeycomb) {
	if config.ApiKey == "" {
		log.Fatalln(color.RedString("ERR_MISSING_HONEYCOMB_API_KEY"))
	}

	if config.ServiceName == "" {
		config.ServiceName = "openregistry-api"
	}

	os.Setenv("OTEL_SERVICE_NAME", config.ServiceName)
	os.Setenv("HONEYCOMB_API_KEY", config.ApiKey)
}
