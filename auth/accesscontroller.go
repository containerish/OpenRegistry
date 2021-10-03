package auth

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	registry_auth "github.com/distribution/distribution/registry/auth"
	"github.com/docker/libtrust"
	"github.com/labstack/echo/v4"
)

//accessSet maps the named resource to a set of actions authorised
type accessSet map[registry_auth.Resource]actionSet

// newAccessSet constructs an accessSet from
// a variable number of auth.Access items.
func newAccessSet(accessItems ...registry_auth.Access) accessSet {
	accessSet := make(accessSet, len(accessItems))

	for _, access := range accessItems {
		resource := registry_auth.Resource{
			Type: access.Type,
			Name: access.Name,
		}
		set, exists := accessSet[resource]
		if !exists {
			set = newActionSet()
			accessSet[resource] = set
		}

		set.add(access.Action)
	}

	return accessSet
}

func (s accessSet) scopeParam() string {
	scopes := make([]string, 0, len(s))

	for resource, actionSet := range s {
		actions := strings.Join(actionSet.keys(), ",")
		scopes = append(scopes, fmt.Sprintf("%s:%s:%s", resource.Type, resource.Name, actions))
	}

	return strings.Join(scopes, " ")
}


type authChallenge struct {
	err          error
	realm        string
	autoRedirect bool
	service      string
	accessSet    accessSet
}

func (ac authChallenge) Error() string {
	return ac.err.Error()
}

func (ac authChallenge) challengeParams(r *http.Request) string {
	var realm string
	if ac.autoRedirect {
		realm = fmt.Sprintf("https://%s/auth/token", r.Host)
	} else {
		realm = ac.realm
	}
	str := fmt.Sprintf("Bearer realm=%q,service=%q", realm, ac.service)
	if scope := ac.accessSet.scopeParam(); scope != "" {
		str = fmt.Sprintf("%s,scope=%q", str, scope)
	}

	if ac.err.Error() == "ErrInvalidToken" || ac.err.Error() == "ErrMalformedToken" {
		str = fmt.Sprintf("%s,error=%q", str, "invalid token")
	} else if ac.err.Error() == "ErrInsufficientScope" {
		str = fmt.Sprintf("%s,error=%q", str, "insufficient_scope")
	}

	return str
}

// SetHeaders sets the WWW-Authenticate value for the response.
func (ac authChallenge) SetHeaders(r *http.Request, w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", ac.challengeParams(r))
}

//tokenAccessOptions is a convenience type for handling
//options to constructor of an accessController
type tokenAccessOptions struct {
	realm          string
	autoRedirect   bool
	issuer         string
	service        string
	rootCertBundle string
}

func checkOptions(options map[string]interface{}) (tokenAccessOptions, error) {
	var opts tokenAccessOptions

	keys := []string{"realm", "issuer", "service", "rootcertbundle"}
	vals := make([]string, 0, len(keys))

	for _, key := range keys {
		val, ok := options[key].(string)
		if !ok {
			return opts, fmt.Errorf("token auth requires a valid option string: %q", key)
		}
		vals = append(vals, val)
	}

	opts.realm, opts.issuer, opts.service, opts.rootCertBundle = vals[0], vals[1], vals[2], vals[3]
	autoRedirectVal, ok := options["autoredirect"]
	if ok {
		autoRedirect, ok := autoRedirectVal.(bool)
		if !ok {
			return opts, fmt.Errorf("token auth requires a valid option bool: autoredirect")
		}
		opts.autoRedirect = autoRedirect
	}
	return opts, nil
}

func newAccessController(options map[string]interface{}) (registry_auth.AccessController, error) {
	config, err := checkOptions(options)
	if err != nil {
		return nil, err
	}

	fp, err := os.Open(config.rootCertBundle)
	if err != nil {
		return nil, fmt.Errorf("unable to open token auth root certificate bundle file %q: %s", config.rootCertBundle, err)
	}
	defer fp.Close()

	rawCertBundle, err := ioutil.ReadAll(fp)
	if err != nil {
		return nil, fmt.Errorf("unable to read token auth root certificate bundle file %q: %s", config.rootCertBundle, err)
	}

	var rootCerts []*x509.Certificate
	pemBlock, rawCertBundle := pem.Decode(rawCertBundle)
	for pemBlock != nil {
		if pemBlock.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(pemBlock.Bytes)
			if err != nil {
				return nil, fmt.Errorf("unable to parse token auth root certificate: %s", err)
			}

			rootCerts = append(rootCerts, cert)
		}
		pemBlock, rawCertBundle = pem.Decode(rawCertBundle)
	}
	if len(rootCerts) == 0 {
		return nil, fmt.Errorf("token auth requires atleast one token signing root certificate")
	}

	rootPool := x509.NewCertPool()
	trustedKeys := make(map[string]libtrust.PublicKey, len(rootCerts))
	for _, rootCert := range rootCerts {
		rootPool.AddCert(rootCert)
		pubKey, err := libtrust.FromCryptoPublicKey(crypto.PublicKey(rootCert.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("unable to get public key from token auth root certificate: %s", err)
		}
		trustedKeys[pubKey.KeyID()] = pubKey
	}

	return &auth{
		store:        nil,
		c:            nil,
		realm:        "",
		autoRedirect: false,
		issuer:       "",
		service:      "",
		rootCerts:    nil,
		trustedKeys:  nil,
	}, nil
}

var (
	ErrNoRequestContext        = errors.New("no http request in context")
	ErrNoResponseWriterContext = errors.New("no http response in context")
	ErrTokenRequired           = errors.New("authorization token required")
	ErrInsufficientScope       = errors.New("insufficient scope")
	ErrInvalidToken		= errors.New("invalid token")
)

// VerifyOptions is used to specify
// options when verifying a JSON Web Token.
type VerifyOptions struct {
	TrustedIssuers    []string
	AcceptedAudiences []string
	Roots             *x509.CertPool
	TrustedKeys       map[string]libtrust.PublicKey
}

// Authorized handles checking whether the given request is authorized
// for actions on resources described by the given access items.
func (a *auth) Authorized(ctx context.Context, accessItems ...registry_auth.Access) (context.Context, error) {
	challenge := a.makeAuthChallenge(accessItems...)

	req, ok := ctx.Value("http.request").(*http.Request)
	if !ok {
		return nil, ErrNoRequestContext
	}

	parts := strings.Split(req.Header.Get("Authorization"), " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		challenge.err = ErrTokenRequired
		return nil, challenge
	}

	token, err := a.verifyToken(parts[2])
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	accessSet := claims.accessSet()
	for _, access := range accessItems {
		if actionSet, ok := accessSet[access.Resource]; !ok {
			if !actionSet.contains(access.Action) {
				challenge.err = ErrInsufficientScope
				return nil, challenge
			}
		}
	}


	return nil, nil
}

func (a *auth) AuthorizedEcho() echo.MiddlewareFunc {
	return func(handlerFunc echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			a.Authorized(c.Request().Context())
			return nil
		}
	}
}

func (a *auth) makeAuthChallenge(access ...registry_auth.Access) authChallenge {
	return authChallenge{
		realm:        a.realm,
		autoRedirect: a.autoRedirect,
		service:      a.service,
		accessSet:    newAccessSet(access...),
	}
}

func (a *auth) verifyToken(rawToken string) (*jwt.Token, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if !token.Valid {
			return nil, errors.New("JWT is invalid")
		}
		return token, nil
	})
	if err != nil {
		return nil, err
	}

	verifyOpts := VerifyOptions{
		TrustedIssuers:    []string{a.issuer},
		AcceptedAudiences: []string{a.service},
		Roots:             a.rootCerts,
		TrustedKeys:       a.trustedKeys,
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	if err := claims.Verify(verifyOpts); err != nil {
		return nil, err
	}

	return token, nil
}