package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const bcryptMinCost = 12

func (a *auth) hashPassword(password string) (string, error) {
	// Convert password string to byte slice
	var passwordBytes = []byte(password)

	// Hash password with Bcrypt's min cost
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword(passwordBytes, bcryptMinCost)

	return string(hashedPasswordBytes), err
}

func (a *auth) verifyPassword(hashedPassword, currPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(currPassword))
	return err == nil
}
