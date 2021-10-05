package telemetry

import (
	"os"

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
	logFmt := "method=${method}, uri=${uri}, status=${status} " +
		"latency=${latency}, bytes_in=${bytes_in}, bytes_out=${bytes_out}\n"

	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(echo.Context) bool {
			return false
		},
		Format: logFmt,
		Output: os.Stdout,
	})
}
