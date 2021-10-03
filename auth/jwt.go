package auth

import (
	"fmt"
	registry_auth "github.com/distribution/distribution/registry/auth"
	"github.com/golang-jwt/jwt"
	"time"
)

type Claims struct {
	jwt.StandardClaims
	Access AccessList
}

func (c *Claims) accessSet() accessSet {
	if c == nil {
		return nil
	}

	set := make(accessSet, len(c.Access))
	for _, action := range c.Access {
		r := registry_auth.Resource{
			Type:  action.Type,
			Name:  action.Name,
		}

		rr, ok := set[r]
		if !ok {
			rr := newActionSet()
			set[r] = rr
		}

		for _, a := range action.Actions {
			rr.add(a)
		}
	}

	return set
}

func (c *Claims) Verify(opts VerifyOptions) error {

	for _,aud := range opts.AcceptedAudiences {
		if !c.VerifyAudience(aud, true){
			return ErrInvalidToken
		}
	}
	now := time.Now()
	tExpiresAt := time.Unix(c.ExpiresAt, 0).Add(time.Minute)
	if tExpiresAt.After(now){
		return ErrInvalidToken
	}
	tNotBefore := time.Unix(c.NotBefore, 0).Add(time.Minute)
	if tNotBefore.Before(now) {
		return ErrInvalidToken
	}

	return nil
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
