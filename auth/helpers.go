package auth

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/golang-jwt/jwt/v5"
)

type CreateCookieOptions struct {
	ExpiresAt   time.Time
	Name        string
	Value       string
	FQDN        string
	Environment config.Environment
	HTTPOnly    bool
}

func CreateCookie(opts *CreateCookieOptions) *http.Cookie {
	secure := true
	sameSite := http.SameSiteNoneMode
	domain := opts.FQDN
	if opts.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		domain = "localhost"
	}

	return &http.Cookie{
		Name:     opts.Name,
		Value:    opts.Value,
		Path:     "/",
		Domain:   domain,
		Expires:  opts.ExpiresAt,
		Secure:   secure,
		SameSite: sameSite,
		HttpOnly: opts.HTTPOnly,
	}
}

const (
	OpenRegistryIssuer = "OpenRegistry"
)

func ReadRSAKeyPair(privKeyPath, pubKeyPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	rawPrivateKey, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, err
	}

	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(rawPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	rawPublicKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, nil, err
	}

	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(rawPublicKey)
	if err != nil {
		return nil, nil, err
	}

	return privKey, pubKey, nil
}

type WebLoginJWTOptions struct {
	Id        string
	Username  string
	TokenType string
	Audience  string
	Privkey   string
	Pubkey    string
}

func NewWebLoginToken(opts *WebLoginJWTOptions) (string, error) {
	acl := AccessList{
		{
			Type:    "repository",
			Name:    fmt.Sprintf("%s/*", opts.Username),
			Actions: []string{"push", "pull"},
		},
	}

	claims := CreateClaims(&CreateClaimOptions{
		Audience: opts.Audience,
		Issuer:   OpenRegistryIssuer,
		Id:       opts.Id,
		TokeType: opts.TokenType,
		Acl:      acl,
	})

	privKey, pubKey, err := ReadRSAKeyPair(opts.Privkey, opts.Pubkey)
	if err != nil {
		return "", err
	}

	pubkeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubkeyDER)
	raw := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	raw.Header["kid"] = KeyIDEncode(hasher.Sum(nil)[:30])
	token, err := raw.SignedString(privKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

type CreateClaimOptions struct {
	Audience string
	Issuer   string
	Id       string
	TokeType string
	Acl      AccessList
}

func CreateClaims(opts *CreateClaimOptions) Claims {
	tokenLife := time.Now().Add(time.Minute * 10)
	switch opts.TokeType {
	case "access":
		// TODO (jay-dee7)
		// token can live for month now, but must be addressed when we implement PASETO
		tokenLife = time.Now().Add(time.Hour * 750)
	case "refresh":
		tokenLife = time.Now().Add(time.Hour * 750)
	case "service":
		tokenLife = time.Now().Add(time.Hour * 750)
	case "short-lived":
		tokenLife = time.Now().Add(time.Minute * 30)
	}

	return Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{opts.Audience},
			ExpiresAt: jwt.NewNumericDate(tokenLife),
			ID:        opts.Id,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    opts.Issuer,
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   opts.Id,
		},
		Access: opts.Acl,
		Type:   opts.TokeType,
	}
}

func KeyIDEncode(b []byte) string {
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
