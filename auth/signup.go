package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/email"
	"github.com/containerish/OpenRegistry/store/postgres"
	"github.com/containerish/OpenRegistry/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (a *auth) SignUp(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	var u types.User
	if err := json.NewDecoder(ctx.Request().Body).Decode(&u); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "error decoding request body in sign-up",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	_ = ctx.Request().Body.Close()

	if err := u.Validate(); err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": "invalid request for user sign up",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if err := verifyPassword(u.Password); err != nil {
		// err.Error() is already user friendly
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error":   err.Error(),
			"message": err.Error(),
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	passwordHash, err := a.hashPassword(u.Password)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "internal server error: could not hash the password",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}
	u.Password = passwordHash

	newUser := &types.User{
		Email:    u.Email,
		Username: u.Username,
		Password: u.Password,
		Id:       uuid.NewString(),
	}

	newUser.Hireable = false
	newUser.HTMLURL = ""

	if a.c.Environment == config.CI {
		newUser.IsActive = true
	}

	err = a.pgStore.AddUser(ctx.Request().Context(), newUser)
	if err != nil {
		if strings.Contains(err.Error(), postgres.ErrDuplicateConstraintUsername) {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "username already exists",
			})
			a.logger.Log(ctx, err)
			return echoErr
		}

		if strings.Contains(err.Error(), postgres.ErrDuplicateConstraintEmail) {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "this email already taken, try sign in?",
			})
			a.logger.Log(ctx, err)
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not persist the user",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	// in case of CI setup, no need to send verification emails
	if a.c.Environment == config.CI {
		msg := fmt.Errorf("user successfully created")
		echoErr := ctx.JSON(http.StatusCreated, echo.Map{
			"message": msg,
		})
		a.logger.Log(ctx, msg)
		return echoErr
	}

	token := uuid.NewString()
	err = a.pgStore.AddVerifyEmail(ctx.Request().Context(), token, newUser.Id)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error adding verify email",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	if err = a.emailClient.SendEmail(newUser, token, email.VerifyEmailKind); err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not send verify link, please reach out to OpenRegistry Team",
		})
		a.logger.Log(ctx, err)
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusCreated, echo.Map{
		"message": "signup was successful, please check your email to activate your account",
	})
	a.logger.Log(ctx, echoErr)
	return echoErr
}

// nolint
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

	if minPassLength > passLen || passLen > maxPassLength {
		appendError(fmt.Sprintf("password length must be between %d to %d characters long", minPassLength, maxPassLength))
	}

	if len(errorString) != 0 {
		return fmt.Errorf(errorString)
	}

	return nil
}
