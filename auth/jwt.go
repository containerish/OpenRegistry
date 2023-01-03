package auth

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/types"
	"github.com/golang-jwt/jwt"
	"golang.org/x/oauth2"
)

type Claims struct {
	jwt.StandardClaims
	Type   string
	Access AccessList
}

type PlatformClaims struct {
	OauthPayload *oauth2.Token `json:"oauth2_token,omitempty"`
	jwt.StandardClaims
	Type string
}

type RefreshClaims struct {
	ID string
	jwt.StandardClaims
}

type ServiceClaims struct {
	jwt.StandardClaims
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

	claims := a.createClaims("public_pull_user", "", acl)

	// TODO (jay-dee7)- handle this properly, check for errors and don't set defaults for actions
	claims.Access[0].Actions = []string{"pull"}

	rawPrivateKey, err := os.ReadFile(a.c.Registry.TLS.PrivateKey)
	if err != nil {
		return "", err
	}

	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}

	rawPublicKey, err := os.ReadFile(a.c.Registry.TLS.PubKey)
	if err != nil {
		return "", err
	}

	pb, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		panic(err)
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(pb)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)

	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	sign, err := token.SignedString(pv)
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

func (a *auth) SignOAuthToken(userId string, payload *oauth2.Token) (string, string, error) {
	return a.newOAuthToken(userId, payload)
}

func (a *auth) newOAuthToken(userId string, payload *oauth2.Token) (string, string, error) {
	accessClaims := a.createOAuthClaims(userId, payload)
	refreshClaims := a.createRefreshClaims(userId)

	rawPrivateKey, err := os.ReadFile(a.c.Registry.TLS.PrivateKey)
	if err != nil {
		return "", "", err
	}
	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}

	rawPublicKey, err := os.ReadFile(a.c.Registry.TLS.PubKey)
	if err != nil {
		return "", "", err
	}

	pb, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		panic(err)
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(pb)
	if err != nil {
		return "", "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	// accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, &accessClaims)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &accessClaims)
	accessToken.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	accessSign, err := accessToken.SignedString(pv)
	if err != nil {
		return "", "", fmt.Errorf("ERR_ACCESS_TOKEN_SIGN: %w", err)
	}

	// refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, &refreshClaims)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &refreshClaims)
	refreshToken.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	refreshSign, err := refreshToken.SignedString(pv)
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
	claims := a.createClaims(u.Id, "service", acl)
	rawPrivateKey, err := os.ReadFile(a.c.Registry.TLS.PrivateKey)
	if err != nil {
		return "", err
	}

	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}

	rawPublicKey, err := os.ReadFile(a.c.Registry.TLS.PubKey)
	if err != nil {
		return "", err
	}

	pb, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		panic(err)
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(pb)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	sign, err := token.SignedString(pv)
	if err != nil {
		return "", fmt.Errorf("error signing secret %w", err)
	}

	return sign, nil
}

func (a *auth) newWebLoginToken(userId, username, tokenType string) (string, error) {
	acl := AccessList{
		{
			Type:    "repository",
			Name:    fmt.Sprintf("%s/*", username),
			Actions: []string{"push", "pull"},
		},
	}
	claims := a.createClaims(userId, tokenType, acl)
	rawPrivateKey, err := os.ReadFile(a.c.Registry.TLS.PrivateKey)
	if err != nil {
		return "", err
	}

	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}

	rawPublicKey, err := os.ReadFile(a.c.Registry.TLS.PubKey)
	if err != nil {
		return "", err
	}

	pb, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		panic(err)
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(pb)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	// raw := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	raw.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])
	token, err := raw.SignedString(pv)
	if err != nil {
		return "", err
	}

	return token, nil
}

// nolint
func (a *auth) createServiceClaims(u types.User) ServiceClaims {
	claims := ServiceClaims{
		StandardClaims: jwt.StandardClaims{
			Audience:  a.c.Endpoint(),
			ExpiresAt: time.Now().Add(time.Hour * 750).Unix(),
			Id:        u.Id,
			IssuedAt:  time.Now().Unix(),
			Issuer:    "OpenRegistry",
			NotBefore: time.Now().Unix(),
			Subject:   u.Id,
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

func (a *auth) createOAuthClaims(userId string, token *oauth2.Token) PlatformClaims {
	claims := PlatformClaims{
		OauthPayload: token,
		StandardClaims: jwt.StandardClaims{
			Audience:  a.c.Endpoint(),
			ExpiresAt: time.Now().Add(time.Hour * 750).Unix(),
			Id:        userId,
			IssuedAt:  time.Now().Unix(),
			Issuer:    "OpenRegistry",
			NotBefore: time.Now().Unix(),
			Subject:   userId,
		},
	}

	return claims
}

func (a *auth) createRefreshClaims(userId string) RefreshClaims {
	claims := RefreshClaims{
		ID: userId,
		StandardClaims: jwt.StandardClaims{
			Audience:  a.c.Endpoint(),
			ExpiresAt: time.Now().Add(time.Hour * 750).Unix(), // Refresh tokens can live longer
			Id:        userId,
			IssuedAt:  time.Now().Unix(),
			Issuer:    "OpenRegistry",
			NotBefore: time.Now().Unix(),
			Subject:   userId,
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
	rawPrivateKey, err := os.ReadFile(a.c.Registry.TLS.PrivateKey)
	if err != nil {
		return "", err
	}

	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}
	claims := a.createClaims(u.Id, "access", acl)

	rawPublicKey, err := os.ReadFile(a.c.Registry.TLS.PubKey)
	if err != nil {
		return "", err
	}

	pb, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		panic(err)
	}

	pubKeyDerBz, err := x509.MarshalPKIXPublicKey(pb)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubKeyDerBz)
	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = a.keyIDEncode(hasher.Sum(nil)[:30])

	// Generate encoded token and send it as response.
	t, err := token.SignedString(pv)
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
func (a *auth) createClaims(id, tokenType string, acl AccessList) Claims {

	tokenLife := time.Now().Add(time.Minute * 10).Unix()
	switch tokenType {
	case "access":
		// TODO (jay-dee7)
		// token can live for month now, but must be addressed when we implement PASETO
		tokenLife = time.Now().Add(time.Hour * 750).Unix()
	case "refresh":
		tokenLife = time.Now().Add(time.Hour * 750).Unix()
	case "service":
		tokenLife = time.Now().Add(time.Hour * 750).Unix()
	case "short-lived":
		tokenLife = time.Now().Add(time.Minute * 30).Unix()
	}

	claims := Claims{
		StandardClaims: jwt.StandardClaims{
			Audience:  a.c.Endpoint(),
			ExpiresAt: tokenLife,
			Id:        id,
			IssuedAt:  time.Now().Unix(),
			Issuer:    "OpenRegistry",
			NotBefore: time.Now().Unix(),
			Subject:   id,
		},
		Access: acl,
		Type:   tokenType,
	}
	return claims
}

type AccessList []struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}
