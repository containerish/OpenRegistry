package auth

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const BcryptMinCost = 6

func (a *auth) hashPassword(password string) (string, error) {
	// Convert password string to byte slice
	var passwordBytes = []byte(password)

	// Hash password with Bcrypt's min cost
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword(passwordBytes, BcryptMinCost)

	return string(hashedPasswordBytes), err
}

func (a *auth) verifyPassword(hashedPassword, currPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(currPassword))
	return err == nil
}

func GenerateSafeHash(input []byte) (string, error) {
	hash := sha256.New()
	if n, err := hash.Write(input); err != nil || n != len(input) {
		return "", fmt.Errorf("error generating hash")
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
