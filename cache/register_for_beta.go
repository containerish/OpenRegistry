package cache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/containerish/OpenRegistry/types"
	"github.com/dgraph-io/badger/v3"
	"github.com/labstack/echo/v4"
)

type BetaRegister struct {
	Emails []string
}

func (br BetaRegister) Bytes() []byte {
	bz, err := json.Marshal(br)
	if err != nil {
		return []byte{}
	}

	return bz
}

const (
	emailRegex = "^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]" +
		"{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"
)

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email can not be empty")
	}
	emailReg := regexp.MustCompile(emailRegex)

	if !emailReg.Match([]byte(email)) {
		return fmt.Errorf("email format invalid")
	}

	return nil
}

func (ds *dataStore) RegisterForBeta(ctx echo.Context) error {

	var body map[string]string
	if err := json.NewDecoder(ctx.Request().Body).Decode(&body); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	err := validateEmail(body["email"])
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid email format, please try again",
		})
	}

	key := []byte("email")
	var value BetaRegister
	list, err := ds.Get([]byte("email"))
	if err != nil || err == badger.ErrKeyNotFound {
		value.Emails = []string{body["email"]}
		if err := ds.Set(key, value.Bytes()); err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}
		return ctx.JSON(http.StatusOK, echo.Map{
			"message": "Success",
		})
	}

	if err = json.Unmarshal(list, &value); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	for _, v := range value.Emails {
		if v == body["email"] {
			return ctx.JSON(http.StatusAlreadyReported, echo.Map{
				"message": "you are already registered for Beta, wait for your cue!",
			})
		}
	}

	value.Emails = append(value.Emails, body["email"])
	if err = ds.Set(key, value.Bytes()); err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, echo.Map{
		"message": "Success",
	})
}

func (ds *dataStore) GetAllEmail(ctx echo.Context) error {
	bz, err := ds.Get([]byte("email"))
	if err != nil && err != badger.ErrKeyNotFound {
		ctx.Set(types.HttpEndpointErrorKey, err.Error())
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": "couldn't get them all",
		})
	}

	return ctx.JSONBlob(http.StatusOK, bz)
}
