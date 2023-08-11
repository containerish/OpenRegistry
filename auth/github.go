package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/containerish/OpenRegistry/config"
	v2_types "github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/go-github/v53/github"
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

	a.mu.Lock()
	a.oauthStateStore[state.String()] = time.Now().Add(time.Minute * 10)
	a.mu.Unlock()
	url := a.github.AuthCodeURL(state.String(), oauth2.AccessTypeOffline)
	echoErr := ctx.Redirect(http.StatusTemporaryRedirect, url)
	a.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (a *auth) GithubLoginCallbackHandler(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	stateToken := ctx.FormValue("state")
	a.mu.Lock()
	_, ok := a.oauthStateStore[stateToken]
	if !ok {
		err := fmt.Errorf("missing or invalid state token")
		uri := a.getGitHubErrorURI(ctx, http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	// no need to compare the stateToken from QueryParam \w stateToken from a.oauthStateStore
	// the key is the actual token :p
	delete(a.oauthStateStore, stateToken)
	a.mu.Unlock()

	code := ctx.FormValue("code")
	token, err := a.github.Exchange(context.Background(), code)
	if err != nil {
		uri := a.getGitHubErrorURI(ctx, http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		uri := a.getGitHubErrorURI(ctx, http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	req.Header.Set("Authorization", "token "+token.AccessToken)
	var ghUser github.User
	_, err = a.ghClient.Do(ctx.Request().Context(), req, &ghUser)
	if err != nil {
		uri := a.getGitHubErrorURI(ctx, http.StatusBadRequest, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	user, err := a.pgStore.GetGitHubUser(ctx.Request().Context(), ghUser.GetEmail(), nil)
	if err != nil {
		user = user.NewUserFromGitHubUser(ghUser)
		err = a.storeGitHubUserIfDoesntExist(ctx.Request().Context(), err, user)
		if err != nil {
			uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusSeeOther, uri)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}
		if err = a.finishGitHubCallback(ctx, user, token); err != nil {
			uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
		a.logger.Log(ctx, nil).Send()
		return err
	}

	if user.WebauthnConnected && !user.GithubConnected {
		err = fmt.Errorf("username/email already exists")
		uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
		echoErr := ctx.Redirect(http.StatusSeeOther, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if user.GithubConnected {
		_, err = a.pgStore.UpdateUser(ctx.Request().Context(), user)
		if err != nil {
			uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusSeeOther, uri)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		if err = a.finishGitHubCallback(ctx, user, token); err != nil {
			uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
			echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
		a.logger.Log(ctx, nil).Send()
		return err
	}

	user.Identities[v2_types.IdentityProviderGitHub] = &v2_types.UserIdentity{
		ID:             fmt.Sprint(ghUser.GetID()),
		Name:           ghUser.GetName(),
		Username:       ghUser.GetLogin(),
		Email:          ghUser.GetEmail(),
		Avatar:         ghUser.GetAvatarURL(),
		InstallationID: 0,
	}

	// this will set the add session object to database and attaches cookies to the echo.Context object
	if err = a.finishGitHubCallback(ctx, user, token); err != nil {
		uri := a.getGitHubErrorURI(ctx, http.StatusConflict, err.Error())
		echoErr := ctx.Redirect(http.StatusTemporaryRedirect, uri)
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = ctx.Redirect(http.StatusTemporaryRedirect, a.c.WebAppConfig.RedirectURL)
	a.logger.Log(ctx, nil).Send()
	return err
}

const (
	AccessCookieMaxAge  = int(time.Second * 3600)
	RefreshCookieMaxAge = int(AccessCookieMaxAge * 3600)
)

func (a *auth) createCookie(
	ctx echo.Context,
	name string,
	value string,
	httpOnly bool,
	expiresAt time.Time,
) *http.Cookie {
	secure := true
	sameSite := http.SameSiteNoneMode
	domain := ""
	url, err := url.Parse(a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment))
	if err != nil {
		domain = a.c.Registry.FQDN
	} else {
		domain = url.Hostname()
	}

	if a.c.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		url, err = url.Parse(a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment))
		if err != nil {
			domain = "localhost"
		} else {
			domain = url.Hostname()
		}
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

	if expiresAt.Unix() < time.Now().Unix() {
		// set cookie deletion
		cookie.MaxAge = -1
	}

	return cookie
}

// makes an http request to get user info from token, if it's valid, it's all good :)
func (a *auth) getUserWithGithubOauthToken(ctx context.Context, token string) (*v2_types.User, error) {
	req, err := a.ghClient.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		return nil, fmt.Errorf("GH_AUTH_REQUEST_ERROR: %w", err)
	}
	req.Header.Set(AuthorizationHeaderKey, "token "+token)

	var oauthUser v2_types.User
	resp, err := a.ghClient.Do(ctx, req, &oauthUser)
	if err != nil {
		return nil, fmt.Errorf("GH_AUTH_ERROR: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GHO_UNAUTHORIZED")
	}

	user, err := a.pgStore.GetUserByEmail(ctx, oauthUser.Email)
	if err != nil {
		return nil, fmt.Errorf("PG_GET_USER_ERR: %w", err)
	}

	return user, nil
}

func (a *auth) getGitHubErrorURI(ctx echo.Context, status int, err string) string {
	queryParams := url.Values{
		"status": {fmt.Sprintf("%d", status)},
		"error":  {err},
	}
	webAppEndoint := a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment)
	return fmt.Sprintf("%s%s?%s", webAppEndoint, a.c.WebAppConfig.ErrorRedirectPath, queryParams.Encode())
}

func (a *auth) finishGitHubCallback(ctx echo.Context, user *v2_types.User, oauthToken *oauth2.Token) error {
	sessionId, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	accessToken, refreshToken, err := a.SignOAuthToken(user.ID, oauthToken)
	if err != nil {
		return err
	}

	err = a.sessionStore.AddSession(
		ctx.Request().Context(),
		sessionId.String(),
		refreshToken,
		user.Identities.GetGitHubIdentity().Username,
	)
	if err != nil {
		return err
	}

	val := fmt.Sprintf("%s:%s", sessionId, user.ID)

	sessionCookie := a.createCookie(ctx, "session_id", val, false, time.Now().Add(time.Hour*750))
	accessCookie := a.createCookie(ctx, "access_token", accessToken, true, time.Now().Add(time.Hour*750))
	refreshCookie := a.createCookie(ctx, "refresh_token", refreshToken, true, time.Now().Add(time.Hour*750))

	ctx.SetCookie(accessCookie)
	ctx.SetCookie(refreshCookie)
	ctx.SetCookie(sessionCookie)

	return nil
}

func (a *auth) storeGitHubUserIfDoesntExist(ctx context.Context, pgErr error, user *v2_types.User) error {
	if errors.Unwrap(pgErr) == pgx.ErrNoRows {
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		user.ID = id.String()
		if err = user.Validate(false); err != nil {
			return err
		}

		// In GitHub's response, Login is the GitHub Username
		if err = a.pgStore.AddUser(ctx, user, nil); err != nil {
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
