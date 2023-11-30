package telemetry

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type Logger interface {
	Log(ctx echo.Context, err error) *zerolog.Event
	Info() *zerolog.Event
	Debug() *zerolog.Event
	DebugWithContext(ctx echo.Context) *zerolog.Event
}

type ZerologOutput interface {
	io.Writer
	Sync() error
}

type logger struct {
	logger zerolog.Logger
	env    config.Environment
}

func ZeroLogger(env config.Environment, config config.Telemetry) Logger {
	baseLogger := setupLogger(config.Logging)

	return &logger{
		logger: baseLogger,
		env:    env,
	}
}

func setupLogger(config config.Logging) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	l := zerolog.New(os.Stdout).With().Caller().Logger()

	logLevel, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	l = l.Output(consoleWriter)

	if config.RemoteForwarding {
		writers := []io.Writer{consoleWriter}
		if config.Axiom.Enabled {
			axiomWriter, err := NewAxiomWriter(
				SetDataset(config.Axiom.Dataset),
				SetClientOptions(
					axiom.SetNoEnv(),
					axiom.SetToken(config.Axiom.APIKey),
					axiom.SetOrganizationID(config.Axiom.OrganizationID),
				),
			)
			if err != nil {
				panic(color.RedString(err.Error()))
			}

			writers = append(writers, axiomWriter)
		}

		if config.FluentBit.Enabled {
			fbWriter := NewFluentBitWriter(&config.FluentBit)
			writers = append(writers, fbWriter)

		}

		levelWriter := zerolog.MultiLevelWriter(writers...)
		l = zerolog.New(levelWriter).With().Caller().Logger()
	}

	return l
}

func (l *logger) Log(ctx echo.Context, errMsg error) *zerolog.Event {
	stop := time.Now()
	start, ok := ctx.Get(types.HandlerStartTime).(time.Time)
	if !ok {
		start = stop
	}
	req := ctx.Request()
	res := ctx.Response()

	level := zerolog.InfoLevel
	status := res.Status
	if status >= 400 {
		level = zerolog.ErrorLevel
	}

	event := l.
		logger.
		WithLevel(level).
		Time("time", start).
		Time("end", stop).
		IPAddr("remote_ip", net.ParseIP(ctx.RealIP())).
		Str("host", req.Host).
		Str("uri", req.RequestURI).
		Str("method", req.Method).
		Str("protocol", req.Proto).
		Str("referer", req.Referer()).
		Str("user_agent", req.UserAgent()).
		Int("status", res.Status).
		Dur("latency", stop.Sub(start)).
		Int64("bytes_out", res.Size).
		Func(func(e *zerolog.Event) {
			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
			}

			e.Str("request_id", id)
		}).
		Func(func(e *zerolog.Event) {
			p := req.URL.Path
			if p == "" {
				p = "/"
			}
			e.Str("path", p)
		}).
		Func(func(e *zerolog.Event) {
			cl := req.Header.Get(echo.HeaderContentLength)
			if cl == "" {
				cl = "0"
			}

			e.Str("bytes_in", cl)
		}).
		Func(func(e *zerolog.Event) {
			if errMsg != nil {
				e.Err(errMsg)
			}
		})

	return event
}

func (l *logger) Debug() *zerolog.Event {
	return l.logger.WithLevel(zerolog.DebugLevel)
}

func (l *logger) Info() *zerolog.Event {
	return l.logger.WithLevel(zerolog.InfoLevel)
}

func (l *logger) DebugWithContext(ctx echo.Context) *zerolog.Event {
	if ctx == nil {
		return l.logger.Debug()
	}

	return l.Log(ctx, nil)
}
