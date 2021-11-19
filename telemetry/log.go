package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	fluentbit "github.com/containerish/OpenRegistry/telemetry/fluent-bit"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/valyala/fasttemplate"
)

func SetupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	l := zerolog.New(os.Stdout)
	l = l.With().Caller().Logger()

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

					if ctxErr, ok := ctx.Get(types.HttpEndpointErrorKey).(string); ok {
						return buf.WriteString(ctxErr)
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

			event := baseLogger.WithLevel(level).RawJSON("msg", buf.Bytes())
			event.Send()
			fluentbitClient.Send(buf.Bytes())
			return
		}
	}
}
