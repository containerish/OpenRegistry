package extensions

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/labstack/echo/v4"
)

func (ext *extension) ChangeContainerImageVisibility(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var body types.ContainerImageVisibilityChangeRequest

	if err := json.NewDecoder(ctx.Request().Body).Decode(&body); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})
	}
	defer ctx.Request().Body.Close()

	err := ext.store.SetContainerImageVisibility(ctx.Request().Context(), body.ImageManifestUUID, body.Visibility)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"message": "container image visibility mode changed successfully",
	})
}
