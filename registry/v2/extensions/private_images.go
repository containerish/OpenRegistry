package extensions

import (
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/labstack/echo/v4"
)

func (ext *extension) ChangeContainerImageVisibility(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var body types.ContainerImageVisibilityChangeRequest

	if err := ctx.Bind(&body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})

		ext.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	err := ext.store.SetContainerImageVisibility(ctx.Request().Context(), body.RepositoryID, body.Visibility)
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
