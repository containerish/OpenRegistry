package auth

import (
	"fmt"
	"time"

	"github.com/containerish/OpenRegistry/store/v1/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type Claims struct {
	jwt.RegisteredClaims
	Type   string
	Access AccessList
}

type OCIClaims struct {
	jwt.RegisteredClaims
	Type   string
	Access types.OCITokenPermissonClaimList
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

func (a *auth) SignOAuthToken(userId uuid.UUID, payload *oauth2.Token) (string, string, error) {
	return a.newOAuthToken(userId, payload)
}

func (a *auth) newOAuthToken(userId uuid.UUID, payload *oauth2.Token) (string, string, error) {
	accessClaims := a.createOAuthClaims(userId, payload)
	refreshClaims := a.createRefreshClaims(userId)

	accessSign, err := a.c.Registry.Auth.SignWithPubKey(&accessClaims)
	if err != nil {
		return "", "", fmt.Errorf("ERR_ACCESS_TOKEN_SIGN: %w", err)
	}

	refreshSign, err := a.c.Registry.Auth.SignWithPubKey(&refreshClaims)
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
		Audience:  a.c.Registry.FQDN,
		Issuer:    OpenRegistryIssuer,
		Id:        u.ID.String(),
		TokenType: "service_token",
		Acl:       acl,
	}
	claims := CreateClaims(opts)
	sign, err := a.c.Registry.Auth.SignWithPubKey(&claims)
	if err != nil {
		return "", fmt.Errorf("error signing secret %w", err)
	}

	return sign, nil
}

func (a *auth) newOCIToken(userID uuid.UUID, scopes types.OCITokenPermissonClaimList) (string, error) {
	opts := &CreateClaimOptionsV2{
		Audience:  a.c.Registry.FQDN,
		Issuer:    OpenRegistryIssuer,
		Id:        userID.String(),
		TokenType: "oci_token",
		Acl:       scopes,
	}
	claims := CreateOCIClaims(opts)
	sign, err := a.c.Registry.Auth.SignWithPubKey(&claims)
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
