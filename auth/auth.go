package auth

import (
	"crypto/x509"
	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/config"
	registry_auth "github.com/distribution/distribution/registry/auth"
	"github.com/docker/libtrust"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Authentication interface {
	SignUp(ctx echo.Context) error
	SignIn(ctx echo.Context) error
	BasicAuth(username, password string) (map[string]interface{}, error)
	registry_auth.AccessController
	registry_auth.CredentialAuthenticator
	registry_auth.Challenge
}

//auth implements the auth.AccessController interface.
type auth struct {
	store        cache.Store
	c            *config.RegistryConfig
	realm        string
	autoRedirect bool
	issuer       string
	service      string
	rootCerts    *x509.CertPool
	trustedKeys  map[string]libtrust.PublicKey
}

func (a *auth) AuthenticateUser(username, password string) error {
	panic("implement me")
}

func (a *auth) Error() string {
	return a.Error()
}

func (a *auth) SetHeaders(r *http.Request, w http.ResponseWriter) {
	panic("implement me")
}

func New(s cache.Store, c *config.RegistryConfig) Authentication {
	accessCtrl := newAccessController()
	a := &auth{store: s, c: c}
	return a
}
