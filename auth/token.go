package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

// Token
// request basically comes for
// https://openregistry.dev/token?service=registry.docker.io&scope=repository:samalba/my-app:pull,push
func (a *auth) Token(ctx echo.Context) error {
	// TODO (jay-dee7) - check for all valid query params here like serive, client_id, offline_token, etc
	// more at this link - https://docs.docker.com/registry/spec/auth/token/
	ctx.Set(types.HandlerStartTime, time.Now())

	authHeader := ctx.Request().Header.Get(AuthorizationHeaderKey)
	if authHeader != "" {
		username, password, err := a.getCredsFromHeader(ctx.Request())
		if err != nil {
			echoErr := ctx.NoContent(http.StatusUnauthorized)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		if strings.HasPrefix(password, "gho_") || strings.HasPrefix(password, "ghp_") {
			user, err := a.getUserWithGithubOauthToken(ctx.Request().Context(), password)
			if err != nil {
				echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
					"error":   err.Error(),
					"message": "invalid github token",
				})
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			token, err := a.newServiceToken(*user)
			if err != nil {
				echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
					"error":   err.Error(),
					"message": "failed to get new service token",
				})
				a.logger.Log(ctx, err).Send()
				return echoErr
			}

			err = ctx.JSON(http.StatusOK, echo.Map{
				"token":      token,
				"expires_in": time.Now().Add(time.Hour).Unix(), // look at auth/jwt.go:251
				"issued_at":  time.Now(),
			})
			a.logger.Log(ctx, err).Send()
			return err
		}

		creds, err := a.validateUser(username, password)
		if err != nil {
			echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error":   err.Error(),
				"message": "error validating user, unauthorised",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		err = ctx.JSON(http.StatusOK, creds)
		a.logger.Log(ctx, err).Send()
		return err
	}

	scope, err := a.getScopeFromQueryParams(ctx.QueryParam("scope"))
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid scope provided",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	// issue a free-public token to pull any repository
	// TODO (jay-dee7) - this should be restricted to only public repositories in the future
	if len(scope.Actions) == 1 && scope.Actions["pull"] {
		token, err := a.newPublicPullToken()
		if err != nil {
			echoErr := ctx.NoContent(http.StatusInternalServerError)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"token": token,
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.NoContent(http.StatusUnauthorized)
	a.logger.Log(ctx, err).Send()
	return err
}

func (a *auth) getCredsFromHeader(r *http.Request) (string, string, error) {
	authHeader := r.Header.Get(AuthorizationHeaderKey)
	if authHeader == "" {
		return "", "", fmt.Errorf("authorization header is missing")
	}

	decodedSting, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(string(decodedSting), ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid creds in Authorization header")
	}

	return parts[0], parts[1], nil
}

func (a *auth) getScopeFromQueryParams(param string) (*Scope, error) {
	parts := strings.Split(param, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid scope in params")
	}

	scope := &Scope{Type: parts[0], Name: parts[1]}
	scope.Actions = make(map[string]bool)

	for _, action := range strings.Split(parts[2], ",") {
		if action != "" {
			scope.Actions[action] = true
		}
	}

	return scope, nil
}

type Scope struct {
	Type    string
	Name    string
	Actions map[string]bool
}
