package telemetry

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/config"

	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/valyala/fasttemplate"
)

type Logger interface {
	Log(ctx echo.Context, err error)
}

func SetupLogger(env string) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	l := zerolog.New(os.Stdout)
	l = l.With().Caller().Logger()
	if env != config.Prod {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			NoColor:    false,
			TimeFormat: time.RFC3339,
		}
		l.Output(consoleWriter)
		return l
	}
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	return l
}

type logger struct {
	fluentBit fluentbit.FluentBit
	output    io.Writer
	pool      *sync.Pool
	template  *fasttemplate.Template
	zlog      zerolog.Logger
	env       string
}

func ZLogger(fluentbitClient fluentbit.FluentBit, env string) Logger {
	pool := &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}
	logFmt := `{"time":"${time_rfc3339}","x_request_id":"${request_id}","remote_ip":"${remote_ip}",` +
		`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
		`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
		`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n"

	baseLogger := SetupLogger(env)

	return &logger{
		zlog:      baseLogger,
		fluentBit: fluentbitClient,
		output:    zerolog.ConsoleWriter{Out: os.Stdout},
		pool:      pool,
		template:  fasttemplate.New(logFmt, "${", "}"),
		env:       env,
	}
}

//nolint:cyclop // insane amount of complexity because of templating
func (l logger) Log(ctx echo.Context, errMsg error) {

	if l.env != config.Prod {
		l.consoleWriter(ctx, errMsg)
		return
	}

	start, ok := ctx.Get("start").(time.Time)
	if !ok {
		start = time.Now()
	}

	stop := time.Now()

	buf := l.pool.Get().(*bytes.Buffer)
	buf.Reset()
	defer l.pool.Put(buf)

	req := ctx.Request()
	res := ctx.Response()

	if errMsg != nil {
		buf.WriteString(errMsg.Error())
	}

	var level zerolog.Level
	if _, err := l.template.ExecuteFunc(buf, func(_ io.Writer, tag string) (int, error) {
		switch tag {
		case "time_rfc3339":
			return buf.WriteString(time.Now().Format(time.RFC3339))
		case "request_id":
			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
			}
			return buf.WriteString(id)
		case "remote_ip":
			return buf.WriteString(ctx.RealIP())
		case "host":
			return buf.WriteString(req.Host)
		case "uri":
			return buf.WriteString(req.RequestURI)
		case "method":
			return buf.WriteString(req.Method)
		case "path":
			p := req.URL.Path
			if p == "" {
				p = "/"
			}
			return buf.WriteString(p)
		case "protocol":
			return buf.WriteString(req.Proto)
		case "referer":
			return buf.WriteString(req.Referer())
		case "user_agent":
			return buf.WriteString(req.UserAgent())
		case "status":
			status := res.Status
			level = zerolog.InfoLevel
			switch {
			case status >= 500:
				level = zerolog.ErrorLevel
			case status >= 400:
				level = zerolog.ErrorLevel
			case status >= 300:
				level = zerolog.WarnLevel
			}

			return buf.WriteString(strconv.FormatInt(int64(status), 10))
		case "latency":
			l := stop.Sub(start)
			return buf.WriteString(strconv.FormatInt(int64(l), 10))
		case "latency_human":
			return buf.WriteString(stop.Sub(start).String())
		case "bytes_in":
			cl := req.Header.Get(echo.HeaderContentLength)
			if cl == "" {
				cl = "0"
			}
			return buf.WriteString(cl)
		case "bytes_out":
			return buf.WriteString(strconv.FormatInt(res.Size, 10))
		default:
			switch {
			case strings.HasPrefix(tag, "header:"):
				return buf.Write([]byte(ctx.Request().Header.Get(tag[7:])))
			case strings.HasPrefix(tag, "query:"):
				return buf.Write([]byte(ctx.QueryParam(tag[6:])))
			case strings.HasPrefix(tag, "form:"):
				return buf.Write([]byte(ctx.FormValue(tag[5:])))
			case strings.HasPrefix(tag, "cookie:"):
				if cookie, cookieErr := ctx.Cookie(tag[7:]); cookieErr == nil {
					return buf.Write([]byte(cookie.Value))
				}
			}
		}
		return 0, nil
	}); err != nil {
		buf.WriteString(fmt.Errorf("templateError: %w", err).Error())
	}

	bz := bytes.TrimSpace(buf.Bytes())
	l.fluentBit.Send(bz)
	l.zlog.WithLevel(level).RawJSON("msg", bz).Send()
}
