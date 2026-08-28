package users

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

type (
	UserApi interface {
		SearchUsers(echo.Context) error
		CreateUserToken(ctx echo.Context) error
		ListUserToken(ctx echo.Context) error
	}

	api struct {
		userStore users.UserStore
		logger    telemetry.Logger
	}
)

func NewApi(userStore users.UserStore, logger telemetry.Logger) UserApi {
	return &api{userStore: userStore, logger: logger}
}

// SearchUsers implements UserApi.
func (a *api) SearchUsers(ctx echo.Context) error {
	query := ctx.QueryParam("query")
	users := make([]*types.User, 0)
	if query == "" {
		echoErr := ctx.JSON(http.StatusOK, users)
		a.logger.Log(ctx, nil).Bool("empty_query", true).Send()
		return echoErr
	}

	users, err := a.userStore.Search(ctx.Request().Context(), query)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, users)
	a.logger.Log(ctx, nil).Send()
	return echoErr
}

func (a *api) CreateUserToken(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		err := fmt.Errorf("missing authentication credentials")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	var body types.CreateAuthTokenRequest
	if err := ctx.Bind(&body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if body.Name == "" {
		err := fmt.Errorf("token name is a required field")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	token, err := types.CreateNewAuthToken()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	hashedToken, err := auth.GenerateSafeHash([]byte(token.RawString()))
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	authToken := &types.AuthTokens{
		CreatedAt: time.Now(),
		ExpiresAt: body.ExpiresAt,
		Name:      body.Name,
		AuthToken: hashedToken,
		OwnerID:   user.ID,
	}

	if err = a.userStore.AddAuthToken(ctx.Request().Context(), authToken); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"token": token.String(),
	})

	a.logger.Log(ctx, nil).Str("client_token", token.String()).Str("stored_token", hashedToken).Send()
	return echoErr
}

func (a *api) ListUserToken(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		err := fmt.Errorf("missing authentication credentials")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	tokens, err := a.userStore.ListAuthTokens(ctx.Request().Context(), user.ID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	if len(tokens) == 0 {
		tokens = make([]*types.AuthTokens, 0)
	}

	echoErr := ctx.JSON(http.StatusOK, tokens)

	a.logger.Log(ctx, nil).Send()
	return echoErr
}
