package telemetry

import (
	"bytes"
	"os"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (l logger) consoleWriter(ctx echo.Context) {
	l.zlog = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC822})
	l.zlog = l.zlog.With().Logger()

	buf := l.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer l.pool.Put(buf)

	req := ctx.Request()
	res := ctx.Response()

	status := res.Status
	level := zerolog.InfoLevel
	switch {
	case status >= 500:
		level = zerolog.ErrorLevel
	case status >= 400:
		level = zerolog.WarnLevel
	case status >= 300:
		level = zerolog.ErrorLevel
	}

	var e multierror.Error

	_, err := buf.WriteString(req.Method + " ")
	e.Errors = append(e.Errors, err)

	_, err = buf.WriteString(color.GreenString("%d ", res.Status))
	e.Errors = append(e.Errors, err)

	if level == zerolog.ErrorLevel {
		e.Errors = append(e.Errors, err)
	}
	if level == zerolog.WarnLevel {
		e.Errors = append(e.Errors, err)
	}

	_, err = buf.WriteString(req.Host)
	e.Errors = append(e.Errors, err)

	_, err = buf.WriteString(req.RequestURI + " ")
	e.Errors = append(e.Errors, err)

	_, err = buf.WriteString(req.Proto + " ")
	e.Errors = append(e.Errors, err)

	_, err = buf.WriteString(req.UserAgent() + " ")
	e.Errors = append(e.Errors, err)

	if ctxErr, ok := ctx.Get(types.HttpEndpointErrorKey).(string); ok {
		_, err = buf.WriteString(color.YellowString(" %s", ctxErr))
		e.Errors = append(e.Errors, err)
	}

	if err := e.ErrorOrNil(); err != nil {
		buf.WriteString(e.Error())
	}

	l.zlog.WithLevel(level).Msg(buf.String())
}
