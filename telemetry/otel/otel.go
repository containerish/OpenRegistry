package otel

import (
	"log"

	"github.com/fatih/color"
	"github.com/honeycombio/otel-config-go/otelconfig"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho" //nolint:staticcheck
	"go.opentelemetry.io/contrib/processors/baggagecopy"
	"go.opentelemetry.io/otel/baggage"

	"github.com/containerish/OpenRegistry/config"
)

func ConfigureOtel(config config.Honeycomb, service string, e *echo.Echo) func() {
	if config.Enabled {
		color.Green("OpenTelemetry: Enabled")
		bsp := baggagecopy.NewSpanProcessor(func(member baggage.Member) bool { return true })

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
