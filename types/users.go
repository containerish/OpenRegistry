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
		CreatedAt         time.Time `json:"created_at,omitempty" validate:"-"`
		UpdatedAt         time.Time `json:"updated_at,omitempty" validate:"-"`
		Id                string    `json:"uuid,omitempty" validate:"-"`
		Password          string    `json:"password,omitempty"`
		Username          string    `json:"username,omitempty" validate:"-"`
		Email             string    `json:"email,omitempty" validate:"email"`
		URL               string    `json:"url,omitempty"`
		Company           string    `json:"company,omitempty"`
		ReceivedEventsURL string    `json:"received_events_url,omitempty"`
		Bio               string    `json:"bio,omitempty"`
		Type              string    `json:"type,omitempty"`
		GravatarID        string    `json:"gravatar_id,omitempty"`
		TwitterUsername   string    `json:"twitter_username,omitempty"`
		HTMLURL           string    `json:"html_url,omitempty"`
		Location          string    `json:"location,omitempty"`
		Login             string    `json:"login,omitempty"`
		Name              string    `json:"name,omitempty"`
		NodeID            string    `json:"node_id,omitempty"`
		OrganizationsURL  string    `json:"organizations_url,omitempty"`
		AvatarURL         string    `json:"avatar_url,omitempty"`
		OAuthID           int       `json:"id,omitempty"`
		IsActive          bool      `json:"is_active,omitempty" validate:"-"`
		Hireable          bool      `json:"hireable,omitempty"`
	}

	OAuthUser struct {
		UpdatedAt         time.Time `json:"updated_at"`
		CreatedAt         time.Time `json:"created_at"`
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

func (u *User) Validate() error {
	if u == nil {
		return fmt.Errorf("user is nil")
	}

	if err := ValidatePassword(u.Password); err != nil {
		return err
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
		appendError("atleast one numeric character required")
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

func (u *User) StripForToken() *User {
	u.CreatedAt = time.Time{}
	u.UpdatedAt = time.Time{}
	u.Password = ""
	u.URL = ""
	u.Company = ""
	u.ReceivedEventsURL = ""
	u.Bio = ""
	u.GravatarID = ""
	u.TwitterUsername = ""
	u.HTMLURL = ""
	u.Location = ""
	u.OrganizationsURL = ""
	u.AvatarURL = ""
	u.Hireable = false

	return u
}
