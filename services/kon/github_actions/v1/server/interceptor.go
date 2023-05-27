package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/containerish/OpenRegistry/vcs/github"
	"github.com/fatih/color"
)

func NewGitHubAppUsernameInterceptor(ghStore vcs.VCSStore, logger telemetry.Logger) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			color.Red("========================= NewGitHubAppUsernameInterceptor ===================================")
			rawCookies := req.Header().Get("cookie")
			header := http.Header{}
			header.Add("Cookie", rawCookies)
			tmpReq := http.Request{Header: header}
			sessionID, err := tmpReq.Cookie("session_id")
			logEvent := logger.Debug().Str("interceptor_name", "NewGitHubAppUsernameInterceptor")
			if err != nil {
				logEvent.Str("error", err.Error()).Send()
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			userID := strings.Split(sessionID.Value, ":")[1]
			user, err := ghStore.GetUserById(ctx, userID, false, nil)
			if err != nil {
				logEvent.Str("error", err.Error()).Send()
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}

			logEvent.Bool("success", true).Send()
			ctx = context.WithValue(ctx, github.UsernameContextKey, user.Username)
			return next(ctx, req)
		})
	})
}

func NewGitHubAppInstallationIDInterceptor(ghStore vcs.VCSStore, skipRoutes []string, logger telemetry.Logger) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			color.Red("========================= NewGitHubAppInstallationIDInterceptor ===================================")
			username, ok := ctx.Value(github.UsernameContextKey).(string)
			logEvent := logger.Debug().Str("interceptor_name", "NewGitHubAppInstallationIDInterceptor")
			if !ok {
				logEvent.Str("missing_value_in_context", string(github.UsernameContextKey))
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("username not found from context"))
			}

			skip := false
			for _, r := range skipRoutes {
				if req.Spec().Procedure == r {
					skip = true
				}
			}

			if skip {
				logEvent.Bool("skip_check", true).Str("Procedure", req.Spec().Procedure)
				return next(ctx, req)
			}

			installationID, err := ghStore.GetInstallationID(ctx, username)
			if err != nil {
				logEvent.Str("error", err.Error())
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}

			logEvent.Bool("success", true).Send()
			ctx = context.WithValue(ctx, github.GithubInstallationIDContextKey, installationID)
			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

type githubAppStreamingInterceptor struct {
	logger       telemetry.Logger
	store        vcs.VCSStore
	routesToSkip []string
}

func NewGithubAppInterceptor(logger telemetry.Logger, store vcs.VCSStore, routesToSkip []string) *githubAppStreamingInterceptor {
	return &githubAppStreamingInterceptor{
		logger,
		store,
		routesToSkip,
	}
}

func (i *githubAppStreamingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		rawCookies := req.Header().Get("cookie")
		header := http.Header{}
		header.Add("Cookie", rawCookies)
		tmpReq := http.Request{Header: header}
		sessionID, err := tmpReq.Cookie("session_id")
		logEvent := i.logger.Debug().Str("interceptor_name", "NewGitHubAppUsernameInterceptor")
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		userID := strings.Split(sessionID.Value, ":")[1]
		user, err := i.store.GetUserById(ctx, userID, false, nil)
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(ctx, github.UsernameContextKey, user.Username)
		skip := false
		for _, r := range i.routesToSkip {
			if req.Spec().Procedure == r {
				skip = true
			}
		}

		if skip {
			logEvent.Bool("skip_check", true).Str("Procedure", req.Spec().Procedure)
			return next(ctx, req)
		}

		installationID, err := i.store.GetInstallationID(ctx, user.Username)
		if err != nil {
			logEvent.Str("error", err.Error())
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(ctx, github.GithubInstallationIDContextKey, installationID)
		return next(ctx, req)
	}
}

func (i *githubAppStreamingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		i.logger.Debug().Str("method", "WrapStreamingClient").Send()
		conn := next(ctx, spec)
		conn.RequestHeader().Set("test-value", "test-value")
		return conn
	}
}

func (i *githubAppStreamingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		logEvent := i.logger.Debug().Str("interceptor_name", "NewGitHubAppUsernameInterceptor").Str("method", "WrapStreamingHandler")

		tmpReq := http.Request{Header: conn.RequestHeader()}
		sessionCookie, err := tmpReq.Cookie("session_id")
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		sessionID, err := url.QueryUnescape(sessionCookie.Value)
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		userID := strings.Split(sessionID, ":")[1]
		user, err := i.store.GetUserById(ctx, userID, false, nil)
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(ctx, github.UsernameContextKey, user.Username)
		skip := false
		for _, r := range i.routesToSkip {
			if conn.Spec().Procedure == r {
				skip = true
			}
		}

		if skip {
			logEvent.Bool("skip_check", true).Str("Procedure", conn.Spec().Procedure)
			return next(ctx, conn)
		}

		installationID, err := i.store.GetInstallationID(ctx, user.Username)
		if err != nil {
			logEvent.Str("error", err.Error())
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(ctx, github.GithubInstallationIDContextKey, installationID)
		return next(ctx, conn)
	}
}
