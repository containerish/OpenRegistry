package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

func (a *auth) LoginWithGithub(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	state, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error generating random id for github login",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	a.oauthStateStore[state.String()] = time.Now().Add(time.Minute * 10)
	url := a.github.AuthCodeURL(state.String(), oauth2.AccessTypeOffline)
	a.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	stateToken := ctx.FormValue("state")
	_, ok := a.oauthStateStore[stateToken]
	if !ok {
		err := fmt.Errorf("INVALID_STATE_TOKEN")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "missing or invalid state token",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	// no need to compare the stateToken from QueryParam \w stateToken from a.oauthStateStore
	// the key is the actual token :p
	delete(a.oauthStateStore, stateToken)

	code := ctx.FormValue("code")
	token, err := a.github.Exchange(context.Background(), code)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "github exchange error",
			"code":    "GITHUB_EXCHANGE_ERR",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		echoErr := ctx.JSON(http.StatusPreconditionFailed, echo.Map{
			"error":   err.Error(),
			"message": "github client request failed",
			"code":    "GH_CLIENT_REQ_FAILED",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	req.Header.Set("Authorization", "token "+token.AccessToken)
	var oauthUser types.User
	_, err = a.ghClient.Do(ctx.Request().Context(), req, &oauthUser)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "github client request execution failed",
			"code":    "GH_CLIENT_REQ_EXEC_FAILED",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	oauthUser.Username = oauthUser.Login
	id, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating oauth user id",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	oauthUser.Id = id.String()

	accessToken, refreshToken, err := a.SignOAuthToken(oauthUser.Id, token)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "JWT_SIGNING",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if err = oauthUser.Validate(false); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if err = a.pgStore.AddOAuthUser(ctx.Request().Context(), &oauthUser); err != nil {
		redirectPath := fmt.Sprintf("%s%s?error=%s", a.c.WebAppEndpoint, a.c.WebAppErrorRedirectPath, err.Error())
		echoErr := ctx.Redirect(http.StatusTemporaryRedirect, redirectPath)
		a.logger.Log(ctx, err)
		return echoErr
	}

	sessionId, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "error creating session id",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	err = a.pgStore.AddSession(ctx.Request().Context(), sessionId.String(), refreshToken, oauthUser.Username)
	if err != nil {
		echoErr := ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppErrorRedirectPath)
		a.logger.Log(ctx, err)
		return echoErr
	}
	val := fmt.Sprintf("%s:%s", sessionId, oauthUser.Id)

	sessionCookie := a.createCookie("session_id", val, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access_token", accessToken, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie("refresh_token", refreshToken, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppRedirectURL)
	a.logger.Log(ctx, nil)
	return err
}

const (
	AccessCookieMaxAge  = int(time.Second * 3600)
	RefreshCookieMaxAge = int(AccessCookieMaxAge * 3600)
)

func (a *auth) createCookie(name string, value string, httpOnly bool, expiresAt time.Time) *http.Cookie {

	secure := true
	sameSite := http.SameSiteNoneMode
	domain := a.c.Registry.FQDN
	if a.c.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		domain = "localhost"
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   domain,
		Expires:  expiresAt,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: httpOnly,
	}
	return cookie
}

// makes an http request to get user info from token, if it's valid, it's all good :)
func (a *auth) getUserWithGithubOauthToken(ctx context.Context, token string) (*types.User, error) {
	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		return nil, fmt.Errorf("GH_AUTH_REQUEST_ERROR: %w", err)
	}
	req.Header.Set(AuthorizationHeaderKey, "token "+token)

	var oauthUser types.User
	resp, err := a.ghClient.Do(ctx, req, &oauthUser)
	if err != nil {
		return nil, fmt.Errorf("GH_AUTH_ERROR: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GHO_UNAUTHORIZED")
	}

	user, err := a.pgStore.GetUser(ctx, oauthUser.Email, false, nil)
	if err != nil {
		return nil, fmt.Errorf("PG_GET_USER_ERR: %w", err)
	}

	return user, nil
}
