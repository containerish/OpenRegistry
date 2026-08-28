package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/common"
	"github.com/containerish/OpenRegistry/registry/v2"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

const (
	OCITokenQueryParamService      = "service"
	OCITokenQueryParamOfflineToken = "offline_token"
	OCITokenQueryParamClientID     = "client_id"
	OCITokenQueryParamScope        = "scope"
	OCITokenQueryParamAccount      = "account"

	// Token lifetimes
	DefaultOCITokenLifetime = time.Minute * 10
)

// Request format: https://openregistry.dev/token?service=registry.docker.io&scope=repository:samalba/my-app:pull,push
func (a *auth) Token(ctx echo.Context) error {
	// TODO (jay-dee7) - check for all valid query params here like serive, client_id, offline_token, etc
	// more at this link - https://docs.docker.com/registry/spec/auth/token/
	ctx.Set(types.HandlerStartTime, time.Now())

	scopes, err := ParseOCITokenPermissionRequest(ctx.Request().URL)
	if err != nil {
		registryErr := common.RegistryErrorResponse(registry.RegistryErrorCodeUnknown, "invalid scope provided", echo.Map{
			"error": err.Error(),
		})
		echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
		a.logger.Log(ctx, registryErr).Send()
		return echoErr
	}

	// when scopes only have one action, and that action is pull
	isPullRequest := len(scopes) == 1 && len(scopes[0].Actions) == 1 && scopes[0].HasPullAccess()
	if isPullRequest {
		repo, repoErr := a.registryStore.GetRepositoryByNamespace(ctx.Request().Context(), scopes[0].Name)
		if repoErr != nil {
			registryErr := common.RegistryErrorResponse(
				registry.RegistryErrorCodeNameInvalid,
				"requested resource does not exist on the registry",
				echo.Map{
					"error": repoErr.Error(),
				},
			)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
			a.logger.Log(ctx, registryErr).Send()
			return echoErr
		}
		user := ctx.Get(string(types.UserContextKey)).(*types.User)

		if repo.Visibility == types.RepositoryVisibilityPublic {
			token, tokenErr := a.newOCIToken(user.ID, scopes)
			if tokenErr != nil {
				registryErr := common.RegistryErrorResponse(
					registry.RegistryErrorCodeNameInvalid,
					"error creating oci token",
					echo.Map{
						"error": tokenErr.Error(),
					},
				)
				echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
				a.logger.Log(ctx, registryErr).Send()
				return echoErr
			}
			now := time.Now()
			echoErr := ctx.JSON(http.StatusOK, echo.Map{
				"token":      token,
				"expires_in": now.Add(DefaultOCITokenLifetime).Unix(),
				"issued_at":  now,
			})
			a.logger.Log(ctx, nil).Send()
			return echoErr
		}
	}

	authHeader := ctx.Request().Header.Get(AuthorizationHeaderKey)
	if authHeader != "" && len(scopes) != 0 {
		token, authErr := a.tryBasicAuthFlow(ctx, scopes)
		if authErr != nil {
			registryErr := common.RegistryErrorResponse(
				registry.RegistryErrorCodeUnauthorized,
				"authentication failed",
				echo.Map{
					"error": authErr.Error(),
				},
			)
			echoErr := ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
			a.logger.Log(ctx, authErr).Send()
			return echoErr
		}
		now := time.Now()
		echoErr := ctx.JSON(http.StatusOK, echo.Map{
			"token":      token,
			"expires_in": now.Add(DefaultOCITokenLifetime).Unix(),
			"issued_at":  now,
		})
		a.logger.Log(ctx, nil).Send()
		return echoErr
	}

	registryErr := common.RegistryErrorResponse(
		registry.RegistryErrorCodeUnauthorized,
		"authentication failed",
		nil,
	)
	err = ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
	a.logger.Log(ctx, registryErr).Send()
	return err
}

func isOCILoginRequest(url *url.URL) types.OCITokenPermissonClaimList {
	account := url.Query().Get(OCITokenQueryParamAccount)
	if account != "" {
		claim := &types.OCITokenPermissonClaim{
			Type:    OCITokenQueryParamAccount,
			Name:    account,
			Actions: []string{OCITokenQueryParamOfflineToken},
		}
		return types.OCITokenPermissonClaimList{claim}
	}

	return nil
}

// Reference format -
// https://auth.cntr.sh/token?service=openregistry.dev&scope=repository:example/my-app:pull,push
func ParseOCITokenPermissionRequest(url *url.URL) (types.OCITokenPermissonClaimList, error) {
	// skip middleware auth in case of login request
	if loginClaims := isOCILoginRequest(url); loginClaims != nil {
		return loginClaims, nil
	}

	// @TODO(jay-dee7) - Maybe we can use this "service" field, once we split registry auth out of this monolith
	// svc := url.Query().Get("service")
	// we use direct map access because a single request can ask for multiple scopes
	scopes := url.Query()[OCITokenQueryParamScope]

	var claimList types.OCITokenPermissonClaimList
	for _, scope := range scopes {
		scopeParts := strings.Split(scope, ":")
		if len(scopeParts) != 3 {
			return nil, fmt.Errorf("ParseOCITokenPermissionRequest: invalid scope")
		}

		scopeType, scopeName := scopeParts[0], scopeParts[1]

		claim := &types.OCITokenPermissonClaim{
			Type:    scopeType,                         // this is usually "repository"
			Name:    scopeName,                         // this is the registry namespace eg: johndoe/ubuntu
			Actions: strings.Split(scopeParts[2], ","), // request action on a resource (push/pull)
		}
		claimList = append(claimList, claim)
	}

	return claimList, nil
}

func (a *auth) loginWithGitHubPAT(
	ctx echo.Context,
	scopes types.OCITokenPermissonClaimList,
	password string,
) (string, error) {
	user, err := a.getUserWithGithubOauthToken(ctx.Request().Context(), password)
	if err != nil {
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   err.Error(),
			"message": "invalid github token",
		})
		a.logger.Log(ctx, err).Send()
		return "", echoErr
	}

	token, err := a.newOCIToken(user.ID, scopes)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "failed to get new service token",
		})
		a.logger.DebugWithContext(ctx).Err(err).Send()
		return "", echoErr
	}

	return token, nil
}

func (a *auth) tryBasicAuthFlow(ctx echo.Context, scopes types.OCITokenPermissonClaimList) (string, error) {
	username, password, err := a.getCredsFromHeader(ctx.Request())
	if err != nil {
		return "", err
	}

	permissions, ok := ctx.Get(string(types.UserPermissionsContextKey)).(*types.Permissions)
	if !ok {
		permissions = &types.Permissions{}
	}

	readOp := ctx.Request().Method == http.MethodGet || ctx.Request().Method == http.MethodHead
	permissonAllowed := permissions.IsAdmin || (readOp && permissions.Pull) || (!readOp && permissions.Push)
	matched := scopes.MatchUsername(username) || scopes.MatchAccount(username)

	if matched || permissonAllowed {
		// try login with GitHub PAT
		// 1. "github_pat_" prefix is for the new fine-grained, repo scoped tokens
		// 2. "ghp_" prefix is for the old (classic) github tokens
		if strings.HasPrefix(password, "github_pat_") || strings.HasPrefix(password, "ghp_") {
			return a.loginWithGitHubPAT(ctx, scopes, password)
		}

		user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
		if !ok {
			registryErr := common.RegistryErrorResponse(
				registry.RegistryErrorCodeUnauthorized,
				"missing authentication info in request",
				echo.Map{
					"error": "missing user authentication info",
				},
			)
			a.logger.Log(ctx, registryErr).Send()
			echoErr := ctx.JSONBlob(http.StatusUnauthorized, registryErr.Bytes())
			return "", echoErr
		}

		token, err := a.newOCIToken(user.ID, scopes)
		if err != nil {
			registryErr := common.RegistryErrorResponse(
				registry.RegistryErrorCodeUnknown,
				"failed to get new service token",
				echo.Map{
					"error": err.Error(),
				},
			)
			echoErr := ctx.JSONBlob(http.StatusBadRequest, registryErr.Bytes())
			a.logger.Log(ctx, registryErr).Send()
			return "", echoErr
		}

		return token, nil
	}

	return "", nil
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

func (a *auth) getScopeFromQueryParams(param string) (*types.Scope, error) {
	parts := strings.Split(param, ":")
	if len(parts) != 3 {
		errMsg := fmt.Errorf("invalid scope in params")
		return nil, errMsg
	}

	scope := &types.Scope{Type: parts[0], Name: parts[1]}
	scope.Actions = make(map[string]bool)

	for _, action := range strings.Split(parts[2], ",") {
		if action != "" {
			scope.Actions[action] = true
		}
	}

	return scope, nil
}
