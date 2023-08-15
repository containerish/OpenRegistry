package auth

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/store/v2/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type Claims struct {
	jwt.RegisteredClaims
	Type   string
	Access AccessList
}

type PlatformClaims struct {
	OauthPayload *oauth2.Token `json:"oauth2_token,omitempty"`
	jwt.RegisteredClaims
	Type string
}

type RefreshClaims struct {
	ID string
	jwt.RegisteredClaims
}

type ServiceClaims struct {
	jwt.RegisteredClaims
	Access AccessList
}

func (a *auth) newPublicPullToken() (string, error) {
	acl := AccessList{
		{
			Type:    "repository",
			Name:    "*/*",
			Actions: []string{"pull"},
		},
	}

	opts := &CreateClaimOptions{
		Audience: a.c.Registry.FQDN,
		Issuer:   OpenRegistryIssuer,
		Id:       "public_pull_user",
		TokeType: "service_token",
		Acl:      acl,
	}

	claims := CreateClaims(opts)

	// TODO (jay-dee7)- handle this properly, check for errors and don't set defaults for actions
	claims.Access[0].Actions = []string{"pull"}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(a.c.Registry.Auth.JWTSigningPubKey)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	sign, err := token.SignedString(a.c.Registry.Auth.JWTSigningPrivateKey)
	if err != nil {
		return "", err
	}

	return sign, nil
}

func (a *auth) keyIDEncode(b []byte) string {
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
	var buf bytes.Buffer
	var i int
	for i = 0; i < len(s)/4-1; i++ {
		start := i * 4
		end := start + 4
		buf.WriteString(s[start:end] + ":")
	}
	buf.WriteString(s[i*4:])
	return buf.String()
}

func (a *auth) SignOAuthToken(userId uuid.UUID, payload *oauth2.Token) (string, string, error) {
	return a.newOAuthToken(userId, payload)
}

func (a *auth) newOAuthToken(userId uuid.UUID, payload *oauth2.Token) (string, string, error) {
	accessClaims := a.createOAuthClaims(userId, payload)
	refreshClaims := a.createRefreshClaims(userId)

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(a.c.Registry.Auth.JWTSigningPubKey)
	if err != nil {
		return "", "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &accessClaims)
	accessToken.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	accessSign, err := accessToken.SignedString(a.c.Registry.Auth.JWTSigningPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("ERR_ACCESS_TOKEN_SIGN: %w", err)
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &refreshClaims)
	refreshToken.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	refreshSign, err := refreshToken.SignedString(a.c.Registry.Auth.JWTSigningPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("ERR_REFRESH_TOKEN_SIGN: %w", err)
	}

	return accessSign, refreshSign, nil
}

// nolint
func (a *auth) newServiceToken(u types.User) (string, error) {
	acl := AccessList{
		{
			Type:    "repository",
			Name:    fmt.Sprintf("%s/*", u.Username),
			Actions: []string{"push", "pull"},
		},
	}
	opts := &CreateClaimOptions{
		Audience: a.c.Registry.FQDN,
		Issuer:   OpenRegistryIssuer,
		Id:       u.ID.String(),
		TokeType: "service_token",
		Acl:      acl,
	}
	claims := CreateClaims(opts)

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(a.c.Registry.Auth.JWTSigningPubKey)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	sign, err := token.SignedString(a.c.Registry.Auth.JWTSigningPrivateKey)
	if err != nil {
		return "", fmt.Errorf("error signing secret %w", err)
	}

	return sign, nil
}

// nolint
func (a *auth) createServiceClaims(u types.User) ServiceClaims {
	claims := ServiceClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{a.c.Endpoint()},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 750)),
			ID:        u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "OpenRegistry",
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   u.ID.String(),
		},
		Access: AccessList{
			{
				Type:    "repository",
				Name:    fmt.Sprintf("%s/*", u.Username),
				Actions: []string{"push", "pull"},
			},
		},
	}

	return claims
}

func (a *auth) createOAuthClaims(userId uuid.UUID, token *oauth2.Token) PlatformClaims {
	claims := PlatformClaims{
		OauthPayload: token,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{a.c.Endpoint()},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 750)),
			ID:        userId.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "OpenRegistry",
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   userId.String(),
		},
	}

	return claims
}

func (a *auth) createRefreshClaims(userId uuid.UUID) RefreshClaims {
	claims := RefreshClaims{
		ID: userId.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{a.c.Endpoint()},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 750)), // Refresh tokens can live longer
			ID:        userId.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "OpenRegistry",
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   userId.String(),
		},
	}

	return claims
}

func (a *auth) newToken(u *types.User) (string, error) {
	//for now we're sending same name for sub and name.
	//TODO when repositories need collaborators

	acl := AccessList{
		{
			Type:    "repository",
			Name:    fmt.Sprintf("%s/*", u.Username),
			Actions: []string{"push", "pull"},
		},
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(a.c.Registry.Auth.JWTSigningPubKey)
	if err != nil {
		return "", err
	}

	opts := &CreateClaimOptions{
		Audience: a.c.Registry.FQDN,
		Issuer:   OpenRegistryIssuer,
		Id:       u.ID.String(),
		TokeType: "access_token",
		Acl:      acl,
	}
	claims := CreateClaims(opts)

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])

	// Generate encoded token and send it as response.
	t, err := token.SignedString(a.c.Registry.Auth.JWTSigningPrivateKey)
	if err != nil {
		return "", err

	}

	return t, nil
}

/*
claims format

	{
	    "iss": "auth.openregistry.dev",
	    "sub": "jlhawn",
	    "aud": "openregistry.dev",
	    "exp": 1415387315,
	    "nbf": 1415387015,
	    "iat": 1415387015,
	    "jti": "tYJCO1c6cnyy7kAn0c7rKPgbV1H1bFws",
	    "access": [
	        {
	            "type": "repository",
	            "name": "samalba/my-app",
	            "actions": [
	                "pull",
	                "push"
	            ]
	        }
	    ]
	}
*/

type AccessList []struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}
