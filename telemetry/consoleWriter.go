package telemetry

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func (l logger) consoleWriter(ctx echo.Context, errMsg error) *zerolog.Event {
	req := ctx.Request()
	res := ctx.Response()

	level := zerolog.InfoLevel
	if res.Status > 299 {
		level = zerolog.ErrorLevel
	}

	event := l.zlog.WithLevel(level)

	event.Str("agent", req.UserAgent())
	event.Str("proto", req.Proto)
	event.Str("host", req.Host)
	event.Str("method", req.Method)
	event.Str("status", fmt.Sprintf("%d", res.Status))
	event.Str("uri", req.RequestURI)
	if errMsg != nil {
		event.Err(errMsg)
	}

	return event
}
