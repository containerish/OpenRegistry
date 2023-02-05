package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/config"
	"github.com/golang-jwt/jwt"
)

func (wa *webauthn_server) createCookie(name string, value string, httpOnly bool, expiresAt time.Time) *http.Cookie {

	secure := true
	sameSite := http.SameSiteNoneMode
	domain := wa.cfg.Registry.FQDN
	if wa.cfg.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		domain = "localhost"
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   domain,
		Expires:  expiresAt,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: httpOnly,
	}
	return cookie
}
func (wa *webauthn_server) newWebLoginToken(userId, username, tokenType string) (string, error) {
	acl := auth.AccessList{
		{
			Type:    "repository",
			Name:    fmt.Sprintf("%s/*", username),
			Actions: []string{"push", "pull"},
		},
	}
	claims := wa.createClaims(userId, tokenType, acl)
	rawPrivateKey, err := os.ReadFile(wa.cfg.Registry.TLS.PrivateKey)
	if err != nil {
		return "", err
	}

	pv, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		panic(err)
	}

	rawPublicKey, err := os.ReadFile(wa.cfg.Registry.TLS.PubKey)
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
	raw := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	raw.Header["kid"] = wa.keyIDEncode(hasher.Sum(nil)[:30])
	token, err := raw.SignedString(pv)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (wa *webauthn_server) createClaims(id, tokenType string, acl auth.AccessList) auth.Claims {

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

	claims := auth.Claims{
		StandardClaims: jwt.StandardClaims{
			Audience:  wa.cfg.Endpoint(),
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

func (wa *webauthn_server) keyIDEncode(b []byte) string {
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
