package otel

import (
	"log"

	"github.com/containerish/OpenRegistry/config"
	"github.com/fatih/color"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/contrib/processors/baggage/baggagetrace"
)

func ConfigureOtel(config config.Telemetry, service string, e *echo.Echo) func() {
	if config.Enabled {
		color.Green("OpenTelemetry: Enabled")
		bsp := baggagetrace.New()

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
