package users

import (
	"net/http"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/store/v1/users"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

type (
	UserApi interface {
		SearchUsers(echo.Context) error
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
