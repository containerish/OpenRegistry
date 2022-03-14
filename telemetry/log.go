package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/config"

	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/rs/zerolog"
	"github.com/valyala/fasttemplate"
)

type Logger interface {
	echo.Logger
	Log(ctx echo.Context)
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

//nolint:cyclop // insane amount of complexity because of templating
func ZerologMiddleware(baseLogger zerolog.Logger, fluentbitClient fluentbit.FluentBit) echo.MiddlewareFunc {
	return func(hf echo.HandlerFunc) echo.HandlerFunc {
		pool := &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 256))
			},
		}

		logFmt := `{"time":"${time_rfc3339}","x_request_id":"${request_id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n"

		template := fasttemplate.New(logFmt, "${", "}")

		return func(ctx echo.Context) (err error) {
			buf := pool.Get().(*bytes.Buffer)
			buf.Reset()
			defer pool.Put(buf)

			req := ctx.Request()
			res := ctx.Response()
			start := time.Now()
			if err = hf(ctx); err != nil {
				ctx.Error(err)
			}
			stop := time.Now()

			var level zerolog.Level
			if _, err = template.ExecuteFunc(buf, func(_ io.Writer, tag string) (int, error) {
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
						level = zerolog.WarnLevel
					case status >= 300:
						level = zerolog.ErrorLevel
					}

					return buf.WriteString(strconv.FormatInt(int64(status), 10))
				case "error":
					if err != nil {
						// Error may contain invalid JSON e.g. `"`
						b, _ := json.Marshal(err.Error())
						b = b[1 : len(b)-1]
						return buf.Write(b)
					}

					if ctxErr, ok := ctx.Get(types.HttpEndpointErrorKey).([]byte); ok {
						return buf.Write(ctxErr)
					}
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
				return
			}

			bz := bytes.TrimSpace(buf.Bytes())
			baseLogger.WithLevel(level).RawJSON("msg", bz).Send()
			fluentbitClient.Send(bz)
			return
		}
	}
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
		output:    os.Stdout,
		pool:      pool,
		template:  fasttemplate.New(logFmt, "${", "}"),
		env:       env,
	}
}

//nolint:cyclop // insane amount of complexity because of templating
func (l logger) Log(ctx echo.Context) {

	if l.env != config.Prod {
		l.consoleWriter(ctx)
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
				level = zerolog.WarnLevel
			case status >= 300:
				level = zerolog.ErrorLevel
			}

			return buf.WriteString(strconv.FormatInt(int64(status), 10))
		case "error":
			if ctxErr, ok := ctx.Get(types.HttpEndpointErrorKey).([]byte); ok {
				return buf.Write(ctxErr)
			}
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

func (l logger) Output() io.Writer {
	return l.output
}

func (l logger) SetOutput(w io.Writer) {
	l.zlog.Output(w)
}

// Prefix is not being used since zerologger is the only logger being used
func (l logger) Prefix() string {
	return ""
}

// SetPrefix is not being used since zerologger is the only logger being used
func (l logger) SetPrefix(p string) {}

func (l logger) Level() log.Lvl {
	level := l.zlog.GetLevel()
	return log.Lvl(level + 1)
}

// SetLevel - echo.loglvl starts from 1 while zerologger starts from 0
func (l logger) SetLevel(v log.Lvl) {
	l.zlog.Level(zerolog.Level(v - 1))
}

func (l logger) SetHeader(h string) {
}

func (l logger) Print(i ...interface{}) {
	l.zlog.WithLevel(l.zlog.GetLevel()).Msgf("%v", i...)
}

func (l logger) Printf(format string, args ...interface{}) {
	l.zlog.WithLevel(l.zlog.GetLevel()).Msgf(format, args...)
}

func (l logger) Printj(j log.JSON) {
	l.zlog.WithLevel(l.zlog.GetLevel()).Fields(j).Send()
}

func (l logger) Debug(i ...interface{}) {
	l.zlog.Debug().Msgf("%v", i...)
}

func (l logger) Debugf(format string, args ...interface{}) {
	l.zlog.Debug().Msgf(format, args...)
}

func (l logger) Debugj(j log.JSON) {
	l.zlog.Debug().Fields(j).Send()
}

func (l logger) Info(i ...interface{}) {
	l.zlog.Info().Msgf("%v", i...)
}

func (l logger) Infof(format string, args ...interface{}) {
	l.zlog.Info().Msgf(format, args...)
}

func (l logger) Infoj(j log.JSON) {
	l.zlog.Info().Fields(j).Send()
}

func (l logger) Warn(i ...interface{}) {
	l.zlog.Warn().Msgf("%v", i...)
}

func (l logger) Warnf(format string, args ...interface{}) {
	l.zlog.Warn().Msgf(format, args...)
}

func (l logger) Warnj(j log.JSON) {
	l.zlog.Warn().Fields(j).Send()
}

func (l logger) Error(i ...interface{}) {
	l.zlog.Error().Msgf("%v", i...)
}

func (l logger) Errorf(format string, args ...interface{}) {
	l.zlog.Error().Msgf(format, args...)
}

func (l logger) Errorj(j log.JSON) {
	l.zlog.Error().Fields(j).Send()
}

func (l logger) Fatal(i ...interface{}) {
	l.zlog.Fatal().Msgf("%v", i...)
}

func (l logger) Fatalj(j log.JSON) {
	l.zlog.Fatal().Fields(j).Send()
}

func (l logger) Fatalf(format string, args ...interface{}) {
	l.zlog.Fatal().Msgf(format, args...)
}

func (l logger) Panic(i ...interface{}) {
	l.zlog.Panic().Msgf("%v", i...)
}

func (l logger) Panicj(j log.JSON) {
	l.zlog.Panic().Fields(j).Send()
}

func (l logger) Panicf(format string, args ...interface{}) {
	l.zlog.Panic().Msgf(format, args...)
}
