package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	"time"
)

func (a *auth) newToken(u User) (string,error) {
	token := jwt.New(jwt.SigningMethodHS256)

	// Set claims
	claims := token.Claims.(jwt.MapClaims)
	claims["username"] = u.Username
	claims["push"] = true
	claims["exp"] = time.Now().Add(time.Hour * 24*14).Unix()

	// Generate encoded token and send it as response.
	fmt.Printf("secret %s:",a.c.SigningSecret)
	t, err := token.SignedString([]byte(a.c.SigningSecret))
	if err != nil {
		return "",err

	}
	return t, nil
}
