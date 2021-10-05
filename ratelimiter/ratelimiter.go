package ratelimiter

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func New() echo.MiddlewareFunc {
	storeConfig := middleware.RateLimiterMemoryStoreConfig{
		Rate:      3,
		Burst:     0,
		ExpiresIn: time.Hour * 10,
	}

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Skipper:    middleware.DefaultSkipper,
		BeforeFunc: nil,
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			return ctx.RealIP(), nil
		},
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(storeConfig),
		ErrorHandler: func(ctx echo.Context, err error) error {
			return ctx.JSON(http.StatusForbidden, echo.Map{"error": "Too many requests, try after some time!"})
		},
		DenyHandler: func(ctx echo.Context, identifier string, err error) error {
			return ctx.JSON(http.StatusForbidden, echo.Map{"error": "Too many requests, try after some time!"})
		},
	})
}
