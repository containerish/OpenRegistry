package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/google/go-github/v50/github"
	"github.com/google/uuid"
)

type (
	User struct {
		UpdatedAt         time.Time  `json:"updated_at,omitempty" validate:"-"`
		CreatedAt         time.Time  `json:"created_at,omitempty" validate:"-"`
		Identities        Identities `json:"identities"`
		Id                string     `json:"uuid,omitempty" validate:"-"`
		Password          string     `json:"password,omitempty"`
		Username          string     `json:"username,omitempty" validate:"-"`
		Email             string     `json:"email,omitempty" validate:"email"`
		IsActive          bool       `json:"is_active,omitempty" validate:"-"`
		WebauthnConnected bool       `json:"webauthn_connected"`
		GithubConnected   bool       `json:"github_connected"`
	}

	Session struct {
		Id           string `json:"id"`
		RefreshToken string `json:"refresh_token"`
		Owner        string `json:"-"`
	}

	Identities   map[string]*UserIdentity
	UserIdentity struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Username       string `json:"username"`
		Email          string `json:"email"`
		Avatar         string `json:"avatar"`
		InstallationID int64  `json:"installation_id"`
	}
)

const IdentityProviderGitHub = "github"
const IdentityProviderWebauthn = "webauthn"

func (i Identities) GetGitHubIdentity() *UserIdentity {
	identity, ok := i[IdentityProviderGitHub]
	if ok {
		return identity
	}

	return nil
}

func (i Identities) GetWebauthnIdentity() *UserIdentity {
	identity, ok := i[IdentityProviderWebauthn]
	if ok {
		return identity
	}

	return nil
}

func (u *User) Validate(validatePassword bool) error {
	if u == nil {
		return fmt.Errorf("user is nil")
	}

	// there's no password for OAuth Users
	if validatePassword {
		if err := ValidatePassword(u.Password); err != nil {
			return fmt.Errorf("invalid password: %w", err)
		}
	}

	v := validator.New()
	return v.Struct(u)
}

func ValidatePassword(password string) error {
	var uppercasePresent bool
	var lowercasePresent bool
	var numberPresent bool
	var specialCharPresent bool
	const minPassLength = 8
	const maxPassLength = 64
	var passLen int
	var errorString string

	for _, ch := range password {
		switch {
		case unicode.IsNumber(ch):
			numberPresent = true
			passLen++
		case unicode.IsUpper(ch):
			uppercasePresent = true
			passLen++
		case unicode.IsLower(ch):
			lowercasePresent = true
			passLen++
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			specialCharPresent = true
			passLen++
		case ch == ' ':
			passLen++
		}
	}
	appendError := func(err string) {
		if len(strings.TrimSpace(errorString)) != 0 {
			errorString += ", " + err
		} else {
			errorString = err
		}
	}
	if !lowercasePresent {
		appendError("lowercase letter missing")
	}
	if !uppercasePresent {
		appendError("uppercase letter missing")
	}
	if !numberPresent {
		appendError("at least one numeric character required")
	}
	if !specialCharPresent {
		appendError("special character missing")
	}

	if minPassLength > passLen || passLen > maxPassLength {
		appendError(fmt.Sprintf("password length must be between %d to %d characters long", minPassLength, maxPassLength))
	}

	if len(errorString) != 0 {
		return fmt.Errorf(errorString)
	}

	return nil
}

func (u *User) Bytes() ([]byte, error) {
	if u == nil {
		return nil, fmt.Errorf("user struct is nil")
	}

	return json.Marshal(u)
}

func (*User) NewUserFromGitHubUser(ghUser github.User) *User {
	return &User{
		Id:              uuid.NewString(),
		Username:        ghUser.GetLogin(),
		Email:           ghUser.GetEmail(),
		IsActive:        true,
		GithubConnected: true,
		Identities: map[string]*UserIdentity{
			IdentityProviderGitHub: {
				ID:             fmt.Sprintf("%d", ghUser.GetID()),
				Name:           ghUser.GetName(),
				Username:       ghUser.GetLogin(),
				Email:          ghUser.GetEmail(),
				Avatar:         ghUser.GetAvatarURL(),
				InstallationID: 0,
			},
		},
	}
}
