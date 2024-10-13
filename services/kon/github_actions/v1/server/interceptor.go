package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/vcs"
	"github.com/containerish/OpenRegistry/vcs/github"
)

type ContextKey string

const (
	OpenRegistryUserContextKey = ContextKey("OPENREGISTRY_USER")
)

func NewGitHubAppUsernameInterceptor(
	ghStore vcs.VCSStore,
	authConfig config.Auth,
	logger telemetry.Logger,
) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			logEvent := logger.Debug().Str("procedure", req.Spec().Procedure)

			userID, err := getTokenFromReq(req, authConfig.JWTSigningPubKey)
			if err != nil {
				return nil, err
			}

			user, err := ghStore.GetUserByID(ctx, userID)
			if err != nil {
				logEvent.Str("error", err.Error()).Send()
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}

			logEvent.Bool("success", true).Send()
			ctx = context.WithValue(ctx, types.UserContextKey, user)
			return next(ctx, req)
		})
	})
}

func PopulateContextWithUserInterceptor(
	ghStore vcs.VCSStore,
	authConfig config.Auth,
	logger telemetry.Logger,
) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			logEvent := logger.Debug().Str("interceptor_name", "NewGitHubAppUsernameInterceptor")
			userID, err := getTokenFromReq(req, authConfig.JWTSigningPubKey)
			if err != nil {
				logEvent.Err(err).Send()
				return nil, err
			}
			user, err := ghStore.GetUserByID(ctx, userID)
			if err != nil {
				logEvent.Err(err).Send()
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}

			logEvent.Bool("success", true).Send()
			ctx = context.WithValue(ctx, types.UserContextKey, user)
			return next(ctx, req)
		})
	})
}

type githubAppStreamingInterceptor struct {
	logger       telemetry.Logger
	store        vcs.VCSStore
	config       *config.Auth
	routesToSkip []string
}

func NewGithubAppInterceptor(
	logger telemetry.Logger, store vcs.VCSStore, routesToSkip []string, config *config.Auth,
) *githubAppStreamingInterceptor {
	return &githubAppStreamingInterceptor{
		logger,
		store,
		config,
		routesToSkip,
	}
}

func (i *githubAppStreamingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		logEvent := i.logger.Debug().Str("Procedure", req.Spec().Procedure)

		userID, err := getTokenFromReq(req, i.config.JWTSigningPubKey)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}

		user, err := i.store.GetUserByID(ctx, userID)
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		ctx = context.WithValue(ctx, OpenRegistryUserContextKey, user)

		logEvent.Bool("success", true)
		ctx = context.WithValue(ctx, types.UserContextKey, user)
		skip := false
		for _, r := range i.routesToSkip {
			if req.Spec().Procedure == r {
				skip = true
			}
		}

		if skip {
			logEvent.Bool("skip_check", true).Str("Procedure", req.Spec().Procedure).Send()
			return next(ctx, req)
		}

		githubIdentity := user.Identities.GetGitHubIdentity()
		if githubIdentity == nil {
			errMsg := fmt.Errorf("github identity is not available")
			logEvent.Err(errMsg).Send()
			return nil, connect.NewError(connect.CodeUnauthenticated, errMsg)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(
			ctx,
			github.GithubInstallationIDContextKey,
			githubIdentity.InstallationID,
		)
		return next(ctx, req)
	}
}

func (i *githubAppStreamingInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		i.logger.Debug().Str("method", "WrapStreamingClient").Send()
		return next(ctx, spec)
	}
}

func (i *githubAppStreamingInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		logEvent := i.logger.Debug().
			Str("interceptor_name", "NewGitHubAppUsernameInterceptor").
			Str("method", "WrapStreamingHandler")

		userID, err := getTokenFromConn(conn, i.config.JWTSigningPubKey, i.logger)
		if err != nil {
			logEvent.Err(err).Send()
			return connect.NewError(connect.CodeUnauthenticated, err)
		}
		user, err := i.store.GetUserByID(ctx, userID)
		if err != nil {
			logEvent.Str("error", err.Error()).Send()
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(ctx, types.UserContextKey, user)
		skip := false
		for _, r := range i.routesToSkip {
			if conn.Spec().Procedure == r {
				skip = true
			}
		}

		if skip {
			logEvent.Bool("skip_check", true).Str("Procedure", conn.Spec().Procedure).Send()
			return next(ctx, conn)
		}

		logEvent.Bool("success", true).Send()
		ctx = context.WithValue(
			ctx,
			github.GithubInstallationIDContextKey,
			user.Identities.GetGitHubIdentity().InstallationID,
		)
		return next(ctx, conn)
	}
}
