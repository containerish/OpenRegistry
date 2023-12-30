package extensions

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/registry"
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/labstack/echo/v4"
)

type Extenion interface {
	CatalogDetail(ctx echo.Context) error
	RepositoryDetail(ctx echo.Context) error
	ChangeContainerImageVisibility(ctx echo.Context) error
	PublicCatalog(ctx echo.Context) error
	GetUserCatalog(ctx echo.Context) error
	AddRepositoryToFavorites(ctx echo.Context) error
	RemoveRepositoryFromFavorites(ctx echo.Context) error
	ListFavoriteRepositories(ctx echo.Context) error
}

type extension struct {
	store  registry.RegistryStore
	logger telemetry.Logger
}

func New(store registry.RegistryStore, logger telemetry.Logger) Extenion {
	return &extension{
		store:  store,
		logger: logger,
	}
}

// CatalogDetail returns a list of container images, goal is to keep it as light as possible
func (ext *extension) CatalogDetail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	namespace := ctx.QueryParam("ns")
	sortBy := ctx.QueryParam("sort_by")
	var pageSize int
	var offset int
	if queryParamPageSize != "" {
		ps, err := strconv.Atoi(ctx.QueryParam("n"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.Atoi(ctx.QueryParam("last"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		offset = o
	}

	total, err := ext.store.GetCatalogCount(ctx.Request().Context(), namespace)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	switch sortBy {
	case "last_updated":
		sortBy = "updated_at desc"
	case "namespace":
		sortBy = "namespace asc"
	case "":
		sortBy = "namespace asc"
	default:
		err = fmt.Errorf("invalid choice of sort_by element")
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	catalogWithDetail, err := ext.store.GetCatalogDetail(
		ctx.Request().Context(),
		namespace,
		pageSize,
		offset,
		sortBy,
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	if catalogWithDetail == nil {
		catalogWithDetail = make([]*types.ContainerImageRepository, 0)
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": catalogWithDetail,
		"total":        total,
	})
	ext.logger.Log(ctx, err).Send()
	return echoErr
}

// RepositoryDetail returns detail of a particular container image
func (ext *extension) RepositoryDetail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	namespace := ctx.QueryParam("ns")
	var pageSize int
	var offset int
	if queryParamPageSize != "" {
		ps, err := strconv.Atoi(ctx.QueryParam("n"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.Atoi(ctx.QueryParam("last"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		offset = o
	}

	repository, err := ext.store.GetRepoDetail(ctx.Request().Context(), namespace, pageSize, offset)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, repository)
	ext.logger.Log(ctx, echoErr).Send()
	return echoErr
}

func (ext *extension) PublicCatalog(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	var pageSize int
	var offset int
	if queryParamPageSize != "" {
		ps, err := strconv.Atoi(ctx.QueryParam("n"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.Atoi(ctx.QueryParam("last"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		offset = o
	}

	repositories, total, err := ext.store.GetPublicRepositories(ctx.Request().Context(), pageSize, offset)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": repositories,
		"total":        total,
	})

	ext.logger.Log(ctx, nil).Send()
	return echoErr
}

func (ext *extension) GetUserCatalog(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, ok := ctx.Get(string(types.UserContextKey)).(*types.User)
	if !ok {
		errMsg := fmt.Errorf("missing user in request context")
		echoErr := ctx.JSON(http.StatusUnauthorized, echo.Map{
			"error":   "unauthorized",
			"message": errMsg.Error(),
		})
		ext.logger.Log(ctx, errMsg).Send()
		return echoErr
	}

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	queryParamVisibility := ctx.QueryParam("visibility")
	var pageSize int
	var offset int
	if queryParamPageSize != "" {
		ps, err := strconv.Atoi(ctx.QueryParam("n"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.Atoi(ctx.QueryParam("last"))
		if err != nil {
			echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
			ext.logger.Log(ctx, err).Send()
			return echoErr
		}
		offset = o
	}

	var visibility types.RepositoryVisibility
	if queryParamVisibility == types.RepositoryVisibilityPublic.String() {
		visibility = types.RepositoryVisibilityPublic
	} else if queryParamVisibility == types.RepositoryVisibilityPrivate.String() {
		visibility = types.RepositoryVisibilityPrivate
	}

	repositories, total, err := ext.store.GetUserRepositories(
		ctx.Request().Context(),
		user.ID,
		visibility,
		pageSize,
		offset,
	)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})

		ext.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"repositories": repositories,
		"total":        total,
	})

	ext.logger.Log(ctx, nil).Send()
	return echoErr
}
