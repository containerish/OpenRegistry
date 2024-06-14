package extensions

import (
	"net/http"
	"time"

	store_v1 "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

func (ext *extension) ChangeContainerImageVisibility(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": store_v1.ErrMissingUserInContext.Error(),
		})

		ext.logger.Log(ctx, store_v1.ErrMissingUserInContext).Send()
		return echoErr
	}

	var body types.ContainerImageVisibilityChangeRequest
	if err := ctx.Bind(&body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	err := ext.store.SetContainerImageVisibility(
		ctx.Request().Context(),
		body.RepositoryID,
		user.ID,
		body.Visibility,
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "container image visibility mode changed successfully",
	})
	ext.logger.Log(ctx, nil).Send()
	return echoErr
}
