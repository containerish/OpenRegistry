package telemetry

import (
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

func SetupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	l := zerolog.New(os.Stdout)
	l.With().Caller().Logger()

	return l
}

func EchoLogger() echo.MiddlewareFunc {
	logFmt := `{"time":"${time_rfc3339_nano}","X-Request-Id":"${id}","remote_ip":"${remote_ip}",` +
		`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
		`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
		`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n"

	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format:           logFmt,
		CustomTimeFormat: time.RFC3339,
		Output:           os.Stdout,
	})
}
