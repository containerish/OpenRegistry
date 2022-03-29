package auth

import (
	"context"
	"fmt"
	"net"
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
	state := uuid.NewString()
	a.oauthStateStore[state] = time.Now().Add(time.Minute * 10)
	url := a.github.AuthCodeURL(state, oauth2.AccessTypeOffline)
	a.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	stateToken := ctx.FormValue("state")
	_, ok := a.oauthStateStore[stateToken]
	if !ok {
		a.logger.Log(ctx, fmt.Errorf("missing or invalid state token"))
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

	oauthUser.Password = refreshToken
	if err = a.pgStore.AddOAuthUser(ctx.Request().Context(), &oauthUser); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"code":  "GH_OAUTH_STORE_OAUTH_USER",
		})
	}

	sessionId := uuid.NewString()
	if err = a.pgStore.AddSession(ctx.Request().Context(), sessionId, refreshToken, oauthUser.Username); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "ERR_CREATING_SESSION",
		})
	}
	val := fmt.Sprintf("%s:%s", sessionId, oauthUser.Id)

	sessionCookie := a.createCookie("session_id", val, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access", accessToken, true, time.Now().Add(time.Hour))
	refreshCookie := a.createCookie("refresh", refreshToken, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)
	a.logger.Log(ctx, nil)
	return ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppRedirectURL)
}

const (
	AccessCookieMaxAge  = int(time.Second * 3600)
	RefreshCookieMaxAge = int(AccessCookieMaxAge * 3600)
)

func (a *auth) createCookie(name string, value string, httpOnly bool, expiresAt time.Time) *http.Cookie {

	secure := true
	sameSite := http.SameSiteStrictMode
	if a.c.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
	}

	webappEndpoint := a.c.WebAppEndpoint
	if a.c.Environment == config.Local {
		host, _, err := net.SplitHostPort(webappEndpoint)
		if err != nil {
			webappEndpoint = host
		}
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   webappEndpoint,
		Expires:  expiresAt,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: httpOnly,
	}
	return cookie
}
