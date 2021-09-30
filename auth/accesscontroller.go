package auth

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	registry_auth "github.com/distribution/distribution/registry/auth"
	"github.com/labstack/echo/v4"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"github.com/docker/libtrust"
)

//accessSet maps the named resource to a set of actions authorised
type accessSet map[registry_auth.Resource]actionSet

// newAccessSet constructs an accessSet from
// a variable number of auth.Access items.
func newAccessSet(accessItems ...registry_auth.Access) accessSet {
	accessSet := make(accessSet, len(accessItems))

	for _, access := range accessItems {
		resource := registry_auth.Resource{
			Type:  access.Type,
			Name:  access.Name,
		}
		set,exists := accessSet[resource]
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

func (ac authChallenge) challengeParams(r *http.Request) string {
	var realm string
	if ac.autoRedirect{
		realm = fmt.Sprintf("https://%s/auth/token", r.Host)
	}else{
		realm = ac.realm
	}
	str := fmt.Sprintf("Bearer realm=%q,service=%q", realm, ac.service)
	if scope := ac.accessSet.scopeParam(); scope != "" {
		str = fmt.Sprintf("%s,scope=%q", str, scope)
	}

	if ac.err.Error()== "ErrInvalidToken" || ac.err.Error() == "ErrMalformedToken" {
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
	realm string
	autoRedirect bool
	issuer string
	service string
	rootCertBundle string
}

func checkOptions(options map[string]interface{}) (tokenAccessOptions, error) {
	var opts tokenAccessOptions

	keys := []string{"realm", "issuer", "service", "rootcertbundle"}
	vals := make([]string, 0, len(keys))

	for _,key := range keys {
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
		store: nil,
		c:     nil,
	}, nil
}



// Authorized handles checking whether the given request is authorized
// for actions on resources described by the given access items.
func (a *auth) Authorized(ctx context.Context, access ...registry_auth.Access) (context.Context, error) {
	if a.realm == "local" {
		return nil, nil
	}

	if a.realm == "akash" {

	}

	return nil,nil
}

func (a *auth) AuthorizedEcho() echo.MiddlewareFunc {
	return func(handlerFunc echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			a.Authorized(c.Request().Context(), )
		}
	}
}

