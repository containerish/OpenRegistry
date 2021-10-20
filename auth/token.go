package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// Token
// request basically comes for
// https://openregistry.dev/token?service=registry.docker.io&scope=repository:samalba/my-app:pull,push
func (a *auth) Token(ctx echo.Context) error {

	// TODO (jay-dee7) - check for all valid query params here like serive, client_id, offline_token, etc
	// more at this link - https://docs.docker.com/registry/spec/auth/token/

	if ctx.Request().Header.Get(AuthorizationHeaderKey) != "" {
		username, password, err := a.getCredsFromHeader(ctx.Request())
		if err != nil {
			return ctx.NoContent(http.StatusUnauthorized)
		}

		creds, err := a.validateUser(username, password)
		if err != nil {
			return ctx.JSON(http.StatusUnauthorized, echo.Map{
				"error": err.Error(),
			})
		}

		return ctx.JSON(http.StatusOK, creds)
	}

	scope, err := a.getScopeFromQueryParams(ctx.QueryParam("scope"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "invalid scope provided",
		})
	}

	if len(scope.Actions) >= 1 && scope.Actions[0] == "pull" {
		token, err := a.newPublicPullToken()
		if err != nil {
			return ctx.NoContent(http.StatusInternalServerError)
		}

		return ctx.JSON(http.StatusOK, echo.Map{
			"token": token,
		})
	}

	return ctx.JSON(http.StatusOK, scope)
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

	return &Scope{
		Type:    parts[0],
		Name:    parts[1],
		Actions: strings.Split(parts[2], ","),
	}, nil
}

type Scope struct {
	Type    string
	Name    string
	Actions []string
}
