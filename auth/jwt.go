package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

type Claims struct {
	jwt.StandardClaims
	Access AccessList
}

func (a *auth) newToken(u User, tokenLife int64) (string, error) {
	//for now we're sending same name for sub and name.
	//TODO when repositories need collaborators
	claims := a.createClaims(u.Username, u.Username, tokenLife)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString([]byte(a.c.SigningSecret))
	if err != nil {
		return "", err

	}

	return t, nil
}

func (a *auth) createClaims(sub, name string, tokenLife int64) Claims {
	claims := Claims{
		StandardClaims: jwt.StandardClaims{
			Audience:  "openregistry.dev",
			ExpiresAt: tokenLife,
			Id:        "",
			IssuedAt:  time.Now().Unix(),
			Issuer:    "openregistry.dev",
			NotBefore: time.Now().Unix(),
			Subject:   sub,
		},
		Access: AccessList{
			{
				Type:    "repository",
				Name:    fmt.Sprintf("%s/*", name),
				Actions: []string{"push", "pull"},
			},
		},
	}

	return claims
}

type AccessList []struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

/*
claims format
{
    "iss": "auth.docker.com",
    "sub": "jlhawn",
    "aud": "registry.docker.com",
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
