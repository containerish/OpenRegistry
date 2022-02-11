package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

func (a *auth) LoginWithGithub(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		a.logger.Log(ctx).Send()
	}()

	url := a.github.AuthCodeURL(a.oauthStateToken, oauth2.AccessTypeOnline)
	return ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	path := ctx.QueryParam("path")
	if path == "" {
		path = a.c.WebAppRedirectURL
	}

	ctx.Set(types.HandlerStartTime, time.Now())
	defer func() {
		a.logger.Log(ctx).Send()
	}()

	stateToken := ctx.FormValue("state")
	if stateToken != a.oauthStateToken {
		ctx.Set(types.HttpEndpointErrorKey, "state token is invalid")
		return ctx.Redirect(http.StatusTemporaryRedirect, "/")
	}

	code := ctx.FormValue("code")
	token, err := a.github.Exchange(context.Background(), code)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"code":  "GITHUB_EXCHANGE_ERR",
		})
	}

	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusPreconditionFailed, echo.Map{
			"error": err.Error(),
			"code":  "GH_CLIENT_REQ_FAILED",
		})
	}

	req.Header.Set("Authorization", "token "+token.AccessToken)
	var oauthUser types.User
	_, err = a.ghClient.Do(ctx.Request().Context(), req, &oauthUser)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"code":  "GH_CLIENT_REQ_EXEC_FAILED",
		})
	}

	oauthUser.Username = oauthUser.Login
	oauthUser.Id = uuid.NewString()

	accessToken, refreshToken, err := a.SignOAuthToken(oauthUser, token)
	if err != nil {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
			"cause": "JWT_SIGNING",
		})
	}

	accessCookie := &http.Cookie{
		Name:    "access",
		Value:   accessToken,
		Expires: time.Now().Add(time.Hour),
		Path:    "/",
	}

	refreshCookie := &http.Cookie{
		Name:    "refresh",
		Value:   refreshToken,
		Expires: time.Now().Add(time.Hour * 750),
		Path:    "/",
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
	redirectURL := fmt.Sprintf("%s%s", a.c.WebAppEndpoint, path)
	return ctx.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
