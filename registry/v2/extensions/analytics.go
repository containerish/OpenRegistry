package extensions

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type FavoriteRepositoryRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" query:"repository_id"`
	UserID       uuid.UUID `json:"user_id" query:"user_id"`
}

func (ext *extension) AddRepositoryToFavorites(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var body FavoriteRepositoryRequest
	if err := ctx.Bind(&body); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	err := ext.store.AddRepositoryToFavorites(ctx.Request().Context(), body.RepositoryID, body.UserID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "repository added to favorites",
	})
	ext.logger.Log(ctx, nil).Send()
	return echoErr
}

func (ext *extension) RemoveRepositoryFromFavorites(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		err := fmt.Errorf("missing authentication credentials")
		echoErr := ctx.JSON(http.StatusForbidden, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	repositoryID, err := uuid.Parse(ctx.Param("repository_id"))
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	body := FavoriteRepositoryRequest{
		RepositoryID: repositoryID,
		UserID:       user.ID,
	}

	err = ext.store.RemoveRepositoryFromFavorites(ctx.Request().Context(), body.RepositoryID, body.UserID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "repository removed from favorites",
	})
	ext.logger.Log(ctx, nil).Send()
	return echoErr
}

func (ext *extension) ListFavoriteRepositories(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		err := fmt.Errorf("missing authentication credentials")
		echoErr := ctx.JSON(http.StatusForbidden, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	repos, err := ext.store.ListFavoriteRepositories(ctx.Request().Context(), user.ID)
	if err != nil {
		repos = make([]*types.ContainerImageRepository, 0)
	}

	echoErr := ctx.JSON(http.StatusOK, repos)

	ext.logger.Log(ctx, nil).Send()
	return echoErr
}
