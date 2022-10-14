package extensions

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

type Extenion interface {
	CatalogDetail(ctx echo.Context) error
	RepositoryDetail(ctx echo.Context) error
}

type extension struct {
	store  postgres.PersistentStore
	logger telemetry.Logger
}

func New(store postgres.PersistentStore, logger telemetry.Logger) (Extenion, error) {
	return &extension{
		store:  store,
		logger: logger,
	}, nil
}

// CatalogDetail returns a list of container images, goal is to keep it as light as possible
func (ext *extension) CatalogDetail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	namespace := ctx.QueryParam("ns")
	sortBy := ctx.QueryParam("sort_by")
	var pageSize int64
	var offset int64
	if queryParamPageSize != "" {
		ps, err := strconv.ParseInt(ctx.QueryParam("n"), 10, 64)
		if err != nil {
			ext.logger.Log(ctx, err).Send()
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.ParseInt(ctx.QueryParam("last"), 10, 64)
		if err != nil {
			ext.logger.Log(ctx, err).Send()
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		offset = o
	}

	total, err := ext.store.GetCatalogCount(ctx.Request().Context(), namespace)
	if err != nil {
		ext.logger.Log(ctx, err).Send()
		return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
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
		ext.logger.Log(ctx, err).Send()
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	catalogWithDetail, err := ext.store.GetCatalogDetail(ctx.Request().Context(), namespace, pageSize, offset, sortBy)
	if err != nil {
		ext.logger.Log(ctx, err).Send()
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	ext.logger.Log(ctx, err).Send()
	return ctx.JSON(http.StatusOK, echo.Map{
		"repositories": catalogWithDetail,
		"total":        total,
	})
}

// RepositoryDetail returns detail of a particular container image
func (ext *extension) RepositoryDetail(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	queryParamPageSize := ctx.QueryParam("n")
	queryParamOffset := ctx.QueryParam("last")
	namespace := ctx.QueryParam("ns")
	var pageSize int64
	var offset int64
	if queryParamPageSize != "" {
		ps, err := strconv.ParseInt(ctx.QueryParam("n"), 10, 64)
		if err != nil {
			ext.logger.Log(ctx, err).Send()
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		pageSize = ps
	}

	if queryParamOffset != "" {
		o, err := strconv.ParseInt(ctx.QueryParam("last"), 10, 64)
		if err != nil {
			ext.logger.Log(ctx, err).Send()
			return ctx.JSON(http.StatusBadRequest, echo.Map{
				"error": err.Error(),
			})
		}
		offset = o
	}

	repository, err := ext.store.GetRepoDetail(ctx.Request().Context(), namespace, pageSize, offset)
	if err != nil {
		ext.logger.Log(ctx, err).Send()
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	ext.logger.Log(ctx, nil).Send()
	return ctx.JSON(http.StatusOK, repository)
}
