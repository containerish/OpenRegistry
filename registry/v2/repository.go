package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type CreateRepositoryRequest struct {
	Name        string                     `json:"name" validate:"required"`
	Description string                     `json:"description" validate:"required"`
	Visibility  types.RepositoryVisibility `json:"visibility"`
}

func (r *CreateRepositoryRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("repository input is nil")
	}

	v := validator.New()
	return v.Struct(r)
}

func (r *registry) CreateRepository(ctx echo.Context) error {
	var body CreateRepositoryRequest
	err := json.NewDecoder(ctx.Request().Body).Decode(&body)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error parsing request input",
		})

		r.logger.Log(ctx, err).Send()
		return echoErr
	}
	defer ctx.Request().Body.Close()

	if err = body.Validate(); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid request body",
		})

		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	user := ctx.Get(string(types.UserContextKey)).(*types.User)
	repository := &types.ContainerImageRepository{
		CreatedAt:   time.Now(),
		ID:          uuid.New(),
		Name:        body.Name,
		Description: body.Description,
		Visibility:  body.Visibility,
		OwnerID:     user.ID,
	}
	if err := r.store.CreateRepository(ctx.Request().Context(), repository); err != nil {
		echoErr := ctx.JSON(http.StatusBadGateway, echo.Map{
			"error":   err.Error(),
			"message": "error creating repository",
		})

		r.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusCreated, echo.Map{
		"message": "repository created successfully",
	})

	r.logger.Log(ctx, nil).Send()
	return echoErr
}
