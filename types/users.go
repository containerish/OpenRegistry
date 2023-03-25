package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"
)

type (
	User struct {
		UpdatedAt         time.Time `json:"updated_at,omitempty" validate:"-"`
		CreatedAt         time.Time `json:"created_at,omitempty" validate:"-"`
		GravatarID        string    `json:"gravatar_id,omitempty"`
		Password          string    `json:"password,omitempty"`
		Id                string    `json:"uuid,omitempty" validate:"-"`
		Username          string    `json:"username,omitempty" validate:"-"`
		Email             string    `json:"email,omitempty" validate:"email"`
		URL               string    `json:"url,omitempty"`
		Company           string    `json:"company,omitempty"`
		ReceivedEventsURL string    `json:"received_events_url,omitempty"`
		HTMLURL           string    `json:"html_url,omitempty"`
		Type              string    `json:"type,omitempty"`
		AvatarURL         string    `json:"avatar_url,omitempty"`
		TwitterUsername   string    `json:"twitter_username,omitempty"`
		Bio               string    `json:"bio,omitempty"`
		Location          string    `json:"location,omitempty"`
		Login             string    `json:"login,omitempty"`
		Name              string    `json:"name,omitempty"`
		NodeID            string    `json:"node_id,omitempty"`
		OrganizationsURL  string    `json:"organizations_url,omitempty"`
		OAuthID           int       `json:"id,omitempty"`
		Hireable          bool      `json:"hireable,omitempty"`
		IsActive          bool      `json:"is_active,omitempty" validate:"-"`
		WebauthnConnected bool      `json:"webauthn_connected"`
		GithubConnected   bool      `json:"github_connected"`
	}

	OAuthUser struct {
		CreatedAt         time.Time `json:"created_at"`
		UpdatedAt         time.Time `json:"updated_at"`
		Location          string    `json:"location"`
		ReceivedEventsURL string    `json:"received_events_url"`
		Email             string    `json:"email"`
		Bio               string    `json:"bio"`
		Type              string    `json:"type"`
		GravatarID        string    `json:"gravatar_id"`
		TwitterUsername   string    `json:"twitter_username"`
		HTMLURL           string    `json:"html_url"`
		Company           string    `json:"company"`
		Login             string    `json:"login"`
		Name              string    `json:"name"`
		NodeID            string    `json:"node_id"`
		OrganizationsURL  string    `json:"organizations_url"`
		AvatarURL         string    `json:"avatar_url"`
		URL               string    `json:"url"`
		FKID              string
		ID                int  `json:"id"`
		Hireable          bool `json:"hireable"`
	}

	Session struct {
		Id           string `json:"id"`
		RefreshToken string `json:"refresh_token"`
		Owner        string `json:"-"`
	}
)

func (u *User) Validate(validatePassword bool) error {
	if u == nil {
		return fmt.Errorf("user is nil")
	}

	// there's no password for OAuth Users
	if validatePassword || u.OAuthID > 0 {
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
