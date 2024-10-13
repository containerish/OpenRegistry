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
	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	// this FQDN is set from the handler which calls this method, using the WebAuthnConfig.GetAllowedURLFromEchoContext
	// method
	domain := opts.FQDN
	if opts.Environment == config.Local {
		secure = false
		sameSite = http.SameSiteLaxMode
		domain = "localhost"
	}

	cookie := &http.Cookie{
		Name:     opts.Name,
		Value:    opts.Value,
		Path:     "/",
		Domain:   domain,
		Expires:  opts.ExpiresAt,
		Secure:   secure,
		HttpOnly: opts.HTTPOnly,
		SameSite: sameSite,
	}

	if opts.ExpiresAt.Unix() < time.Now().Unix() {
		cookie.MaxAge = -1
	}

	return cookie
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
	Privkey   *rsa.PrivateKey
	Pubkey    *rsa.PublicKey
	Username  string
	TokenType string
	Audience  string
	Id        uuid.UUID
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
		Audience:  opts.Audience,
		Issuer:    OpenRegistryIssuer,
		Id:        opts.Id.String(),
		TokenType: opts.TokenType,
		Acl:       acl,
	})

	pubkeyDER, err := x509.MarshalPKIXPublicKey(opts.Pubkey)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(pubkeyDER)
	raw := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
	raw.Header["kid"] = KeyIDEncode(hasher.Sum(nil)[:30])
	token, err := raw.SignedString(opts.Privkey)
	if err != nil {
		return "", err
	}

	return token, nil
}

type CreateClaimOptionsV2 struct {
	Audience  string
	Issuer    string
	Id        string
	TokenType string
	Acl       types.OCITokenPermissonClaimList
}

type CreateClaimOptions struct {
	Audience  string
	Issuer    string
	Id        string
	TokenType string
	Acl       AccessList
}

func CreateClaims(opts *CreateClaimOptions) Claims {
	now := time.Now()
	iat := jwt.NewNumericDate(now)
	nbf := iat
	var tokenLife time.Time

	switch opts.TokenType {
	case AccessCookieKey, RefreshCookKey, Service:
		// TODO (jay-dee7)
		// token can live for month now, but must be addressed when we implement PASETO
		tokenLife = now.Add(time.Hour * 750)
	default:
		tokenLife = now.Add(time.Minute * 10)
	}

	return Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{opts.Audience},
			ExpiresAt: jwt.NewNumericDate(tokenLife),
			ID:        opts.Id,
			IssuedAt:  iat,
			Issuer:    opts.Issuer,
			NotBefore: nbf,
			Subject:   opts.Id,
		},
		Access: opts.Acl,
		Type:   opts.TokenType,
	}
}

func CreateOCIClaims(opts *CreateClaimOptionsV2) OCIClaims {
	now := time.Now()
	iat := jwt.NewNumericDate(now)
	nbf := iat
	var tokenLife time.Time

	switch opts.TokenType {
	case AccessCookieKey, RefreshCookKey, Service:
		// TODO (jay-dee7)
		// token can live for month now, but must be addressed when we implement PASETO
		tokenLife = now.Add(time.Hour * 750)
	default:
		tokenLife = now.Add(time.Minute * 10)
	}

	return OCIClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        opts.Id,
			Issuer:    opts.Issuer,
			Subject:   opts.Id,
			ExpiresAt: jwt.NewNumericDate(tokenLife),
			IssuedAt:  iat,
			NotBefore: nbf,
			Audience:  jwt.ClaimStrings{opts.Audience},
		},
		Access: opts.Acl,
		Type:   opts.TokenType,
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
