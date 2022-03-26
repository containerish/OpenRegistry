package telemetry

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (l logger) consoleWriter(ctx echo.Context, errMsg error) {
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

	var e error

	_, err := buf.WriteString(req.Method + " ")
	e = multierror.Append(e, err)

	_, err = buf.WriteString(color.GreenString("%d ", res.Status))
	e = multierror.Append(e, err)

	if level == zerolog.ErrorLevel {
		e = multierror.Append(e, err)
	}
	if level == zerolog.WarnLevel {
		e = multierror.Append(e, err)
	}

	_, err = buf.WriteString(req.Host)
	e = multierror.Append(e, err)

	_, err = buf.WriteString(req.RequestURI + " ")
	e = multierror.Append(e, err)

	_, err = buf.WriteString(req.Proto + " ")
	e = multierror.Append(e, err)

	_, err = buf.WriteString(req.UserAgent() + " ")
	e = multierror.Append(e, err)

	if errMsg != nil {
		_, err = buf.WriteString(color.YellowString(" %s", errMsg))
		e = multierror.Append(e, err)
	}

	merr := e.(*multierror.Error)
	if merr.ErrorOrNil() != nil {
		buf.WriteString(strings.TrimSpace(merr.Error()))
	}

	l.zlog.WithLevel(level).Msg(buf.String())
}
