package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/containerish/OpenRegistry/cache"
	"github.com/containerish/OpenRegistry/types"
	"github.com/labstack/echo/v4"
)

type User struct {
	Id       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

const UserNameSpace = "users"

func (u *User) Validate(store cache.Store) error {

	if err := verifyEmail(u.Email); err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%s", UserNameSpace, u.Email)
	_, err := store.Get([]byte(key))
	if err == nil {
		return fmt.Errorf("user already exists, try loggin in or password reset")
	}

	if len(u.Username) < 3 {
		return fmt.Errorf("username should be atleast 3 chars")
	}

	bz, err := store.ListWithPrefix([]byte(UserNameSpace))
	if err != nil {
		return fmt.Errorf("internal server error")
	}

	if bz != nil {
		var userList []User
		if err := json.Unmarshal(bz, &userList); err != nil {

			if strings.Contains(err.Error(), "object into Go value of type []auth.User") {
				var usr User
				if e := json.Unmarshal(bz, &usr); e != nil {
					return e
				}
				userList = append(userList, usr)
			} else {
				return fmt.Errorf("error in unmarshaling: %w", err)
			}
		}

		for _, user := range userList {
			if u.Username == user.Username {
				return fmt.Errorf("username already taken")
			}
		}
	}
	return verifyPassword(u.Password)
}

func verifyEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email can not be empty")
	}
	emailReg := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}" +
		"[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	if !emailReg.Match([]byte(email)) {
		return fmt.Errorf("email format invalid")
	}

	return nil
}

func verifyPassword(password string) error {
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
	if !(minPassLength <= passLen && passLen <= maxPassLength) {
		appendError(fmt.Sprintf("password length must be between %d to %d characters long", minPassLength, maxPassLength))
	}

	if len(errorString) != 0 {
		return fmt.Errorf(errorString)
	}
	return nil
}

func (a *auth) SignUp(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var u User
	if err := json.NewDecoder(ctx.Request().Body).Decode(&u); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error decoding request body in sign-up",
		})
	}
	_ = ctx.Request().Body.Close()

	if err := u.Validate(a.store); err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	hpwd, err := a.hashPassword(u.Password)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}
	u.Password = hpwd

	newUser := &types.User{
		Email:    u.Email,
		Username: u.Username,
		Password: u.Password,
	}

	err = a.pgStore.AddUser(ctx.Request().Context(), newUser)
	if err != nil {
		a.logger.Log(ctx, err)
		return ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error": err.Error(),
		})
	}

	accessToken, refreshToken, err := a.newWebLoginToken(*newUser)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
			"code":  "CREATE_NEW_TOKEN",
		})
	}

	accessCookie := &http.Cookie{
		Name:    "access",
		Value:   accessToken,
		Expires: time.Now().Add(time.Hour),
		Path:    "/",
	}

	refreshCookie := &http.Cookie{
		Name:    "refresh",
		Value:   refreshToken,
		Expires: time.Now().Add(time.Hour * 750),
		Path:    "/",
	}

	http.SetCookie(ctx.Response(), accessCookie)
	http.SetCookie(ctx.Response(), refreshCookie)
	a.logger.Log(ctx, err)
	return ctx.JSON(http.StatusCreated, echo.Map{
		"message": "user successfully created",
	})
}
