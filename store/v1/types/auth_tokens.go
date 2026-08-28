package types

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type AuthToken struct {
	id ulid.ULID
}

const (
	OpenRegistryAuthTokenPrefix = "oreg_pat_"
)

func (at *AuthToken) String() string {
	return fmt.Sprintf("%s%s", OpenRegistryAuthTokenPrefix, at.id.String())
}

func (at *AuthToken) Bytes() []byte {
	return at.id.Bytes()
}

func (at *AuthToken) Compare(other ulid.ULID) int {
	return at.id.Compare(other)
}

func (at *AuthToken) FromString(token string) (*AuthToken, error) {
	token = strings.TrimPrefix(token, OpenRegistryAuthTokenPrefix)

	id, err := ulid.Parse(token)
	if err != nil {
		return nil, err
	}

	return &AuthToken{id: id}, nil
}

func (at *AuthToken) RawString() string {
	return at.id.String()
}

func CreateNewAuthToken() (*AuthToken, error) {
	now := time.Now()
	ms := ulid.Timestamp(now)
	id, err := ulid.New(ms, rand.Reader)
	if err != nil {
		return nil, err
	}

	return &AuthToken{
		id: id,
	}, nil
}
