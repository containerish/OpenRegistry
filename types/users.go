package types

import (
	"time"

	"github.com/go-playground/validator/v10"
)

type (
	User struct {
		Id        string    `json:"id" validate:"-"`
		Email     string    `json:"email" validate:"email"`
		Username  string    `json:"username" validate:"gte=4"`
		Password  string    `json:"password" validate:"required,gte=8"`
		CreatedAt time.Time `json:"created_at" validate:"-"`
		UpdatedAt time.Time `json:"updated_at" validate:"-"`
		IsActive  bool      `json:"is_active" validate:"-"`
	}
)

func (u *User) Validate() error {
	v := validator.New()

	return v.Struct(u)
}
