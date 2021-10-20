package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/labstack/echo/v4"
)

// Token
// request basically comes for
// https://openregistry.dev/token?service=registry.docker.io&scope=repository:samalba/my-app:pull,push
func (a *auth) Token(ctx echo.Context) error {
	// service := ctx.QueryParam("service")
	// if _, ok := a.c.AuthConfig.SupportedServices[service]; !ok {
	// 	return ctx.JSON(http.StatusBadRequest, echo.Map{
	// 		"error": fmt.Sprintf("%s service is not supported by OpenRegistry", service),
	// 	})
	// }

	scope, err := a.getScopeFromQueryParams(ctx.QueryParam("scope"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"msg":   "invalid scope provided",
		})
	}

	if scope.Actions[0] == "pull" {
		return ctx.NoContent(http.StatusOK)
	}

	color.Red("scope from request: %v", scope)
	return ctx.JSON(http.StatusOK, scope)
}

func (a *auth) getScopeFromQueryParams(param string) (*Scope, error) {
	parts := strings.Split(param, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid scope in params")
	}

	return &Scope{
		Type:    parts[0],
		Name:    parts[1],
		Actions: strings.Split(parts[2], ","),
	}, nil
}

type Scope struct {
	Type    string
	Name    string
	Actions []string
}
