package otel

import (
	"log"

	"github.com/containerish/OpenRegistry/config"
	"github.com/fatih/color"
	"github.com/honeycombio/honeycomb-opentelemetry-go"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func ConfigureOtel(config config.Telemetry, service string, e *echo.Echo) func() {
	if config.Enabled {
		color.Green("OpenTelemetry: Enabled")
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
