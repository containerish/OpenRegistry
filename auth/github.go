package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
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
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	a.oauthStateStore[state.String()] = time.Now().Add(time.Minute * 10)
	url := a.github.AuthCodeURL(state.String(), oauth2.AccessTypeOffline)
	echoErr := ctx.Redirect(http.StatusTemporaryRedirect, url)
	a.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	stateToken := ctx.FormValue("state")
	_, ok := a.oauthStateStore[stateToken]
	if !ok {
		err := fmt.Errorf("missing or invalid state token")
		uri := a.getGitHubErrorURI(http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	// no need to compare the stateToken from QueryParam \w stateToken from a.oauthStateStore
	// the key is the actual token :p
	delete(a.oauthStateStore, stateToken)

	code := ctx.FormValue("code")
	token, err := a.github.Exchange(context.Background(), code)
	if err != nil {
		uri := a.getGitHubErrorURI(http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		uri := a.getGitHubErrorURI(http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	req.Header.Set("Authorization", "token "+token.AccessToken)
	var oauthUser types.User
	_, err = a.ghClient.Do(ctx.Request().Context(), req, &oauthUser)
	if err != nil {
		uri := a.getGitHubErrorURI(http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	user, err := a.pgStore.GetOAuthUser(ctx.Request().Context(), oauthUser.Login, nil)
	if err != nil {
		err = a.storeGitHubUserIfDoesntExist(ctx.Request().Context(), err, &oauthUser)
		if err != nil {
			uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusSeeOther, uri)
			a.logger.Log(ctx, err)
			return echoErr
		}
		if err = a.finishGitHubCallback(ctx, oauthUser.Username, oauthUser.Id, token); err != nil {
			uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
			a.logger.Log(ctx, err)
			return echoErr
		}

		err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
		a.logger.Log(ctx, nil)
		return err
	}
	if user.WebauthnConnected && !user.GithubConnected {
		err = fmt.Errorf("username/email already exists")
		uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	if user.GithubConnected {
		err = a.pgStore.UpdateOAuthUser(
			ctx.Request().Context(),
			oauthUser.Email,
			oauthUser.Login,
			oauthUser.NodeID,
			nil,
		)
		if err != nil {
			uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusSeeOther, uri)
			a.logger.Log(ctx, err)
			return echoErr
		}

		if err = a.finishGitHubCallback(ctx, oauthUser.Login, user.Id, token); err != nil {
			uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
			a.logger.Log(ctx, err)
			return echoErr
		}

		err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
		a.logger.Log(ctx, nil)
		return err
	}

	// this will set the add session object to database and attaches cookies to the echo.Context object
	if err = a.finishGitHubCallback(ctx, oauthUser.Username, oauthUser.Id, token); err != nil {
		uri := a.getGitHubErrorURI(http.StatusConflict, err.Error())
		echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
		a.logger.Log(ctx, err)
		return echoErr
	}

	err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
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

func (a *auth) getGitHubErrorURI(status int, err string) string {
	queryParams := url.Values{
		"status": {fmt.Sprintf("%d", status)},
		"error":  {err},
	}

	return fmt.Sprintf("%s%s?%s", a.c.WebAppConfig.Endpoint, a.c.WebAppConfig.CallbackURL, queryParams.Encode())
}

func (a *auth) finishGitHubCallback(ctx echo.Context, username, userId string, oauthToken *oauth2.Token) error {
	sessionId, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	accessToken, refreshToken, err := a.SignOAuthToken(userId, oauthToken)
	if err != nil {
		return err
	}

	err = a.pgStore.AddSession(ctx.Request().Context(), sessionId.String(), refreshToken, username)
	if err != nil {
		return err
	}

	val := fmt.Sprintf("%s:%s", sessionId, userId)

	sessionCookie := a.createCookie("session_id", val, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie("access_token", accessToken, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie("refresh_token", refreshToken, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	return nil
}

func (a *auth) storeGitHubUserIfDoesntExist(ctx context.Context, pgErr error, user *types.User) error {
	if errors.Unwrap(pgErr) == pgx.ErrNoRows {
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		user.Id = id.String()
		if err = user.Validate(false); err != nil {
			return err
		}

		// In GitHub's response, Login is the GitHub Username
		user.Username = user.Login
		user.GithubConnected = true
		if err = a.pgStore.AddOAuthUser(ctx, user); err != nil {
			var pgErr *pgconn.PgError
			// this would mean that the user email is already registered
			// so we return an error in this case
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				return fmt.Errorf("username/email already exists")
			}
			return err
		}
		return nil
	}

	return pgErr
}
