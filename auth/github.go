package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

func (a *auth) LoginWithGithub(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	state := uuid.NewString()
	a.oauthStateStore[state] = time.Now().Add(time.Minute * 10)
	url := a.github.AuthCodeURL(state, oauth2.AccessTypeOffline)
	a.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	path := ctx.QueryParam("path")
	if path == "" {
		path = a.c.WebAppRedirectURL
	}
	ctx.Set(types.HandlerStartTime, time.Now())

	stateToken := ctx.FormValue("state")
	_, ok := a.oauthStateStore[stateToken]
	if !ok {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "missing or invalid state token",
		})
	}
	// no need to compare the stateToken from QueryParam \w stateToken from a.oauthStateStore
	// the key is the actual token :p
	delete(a.oauthStateStore, stateToken)

	code := ctx.FormValue("code")
	token, err := a.github.Exchange(context.Background(), code)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"code":  "GITHUB_EXCHANGE_ERR",
		})
	}

	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusPreconditionFailed, echo.Map{
			"error": err.Error(),
			"code":  "GH_CLIENT_REQ_FAILED",
		})
	}

	req.Header.Set("Authorization", "token "+token.AccessToken)
	var oauthUser types.User
	_, err = a.ghClient.Do(ctx.Request().Context(), req, &oauthUser)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"code":  "GH_CLIENT_REQ_EXEC_FAILED",
		})
	}

	oauthUser.Username = oauthUser.Login
	oauthUser.Id = uuid.NewString()

	accessToken, refreshToken, err := a.SignOAuthToken(oauthUser, token)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "JWT_SIGNING",
		})
	}

	secure := true
	sameSite := http.SameSiteStrictMode
	domain := strings.TrimPrefix(a.c.WebAppEndpoint, "https://")
	if a.c.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		domain = "localhost"
	}

	accessCookie := &http.Cookie{
		Name:     "access",
		Value:    accessToken,
		Path:     "/",
		Domain:   domain,
		Expires:  time.Now().Add(time.Hour),
		MaxAge:   AccessCookieMaxAge,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: true,
	}

	refreshCookie := &http.Cookie{
		Name:     "refresh",
		Value:    refreshToken,
		Path:     "/",
		Domain:   domain,
		Expires:  time.Now().Add(time.Hour * 750),
		MaxAge:   RefreshCookieMaxAge,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: true,
	}

	oauthUser.Password = refreshToken
	if err := a.pgStore.AddOAuthUser(ctx.Request().Context(), &oauthUser); err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"code":  "GH_OAUTH_STORE_OAUTH_USER",
		})
	}

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	redirectURL := a.c.WebAppEndpoint + path
	a.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

const (
	AccessCookieMaxAge  = int(time.Second * 3600)
	RefreshCookieMaxAge = int(AccessCookieMaxAge * 3600)
)
