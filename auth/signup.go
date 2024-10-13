package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/containerish/OpenRegistry/config"
	"github.com/containerish/OpenRegistry/services/email"
	store_err "github.com/containerish/OpenRegistry/store/v1"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

func (a *auth) parseSignUpRequest(ctx echo.Context) (*types.User, error) {
	var user types.User
	if err := json.NewDecoder(ctx.Request().Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("error parsing signup request: %w", err)
	}
	defer ctx.Request().Body.Close()

	if user.UserType != "" &&
		user.UserType != types.UserTypeRegular.String() &&
		user.UserType != types.UserTypeOrganization.String() {
		err := fmt.Errorf("invalid user type. Supported values are: user, organization")
		return nil, err
	}

	// by default all users are regular users
	if user.UserType == "" {
		user.UserType = types.UserTypeRegular.String()
	}

	if err := user.Validate(true); err != nil {
		return nil, err
	}

	passwordHash, err := a.hashPassword(user.Password)
	if err != nil {
		return nil, err
	}
	user.Password = passwordHash

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	newUser := &types.User{
		ID:        id,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
		Password:  user.Password,
		Username:  user.Username,
		Email:     user.Email,
		UserType:  user.UserType,
	}

	skipEmailVerification := !a.c.Email.Enabled || (a.c.Environment == config.Local || a.c.Environment == config.CI)
	// no need to do email verification in local mode or if the email service is disabled
	if skipEmailVerification {
		newUser.IsActive = true
	}

	return newUser, nil
}

func (a *auth) SignUp(ctx echo.Context) error {
	ctx.Set(types.HandlerStartTime, time.Now())

	user, err := a.parseSignUpRequest(ctx)
	if err != nil {
		echoErr := ctx.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	err = a.userStore.AddUser(ctx.Request().Context(), user, nil)
	if err != nil {
		if strings.Contains(err.Error(), store_err.ErrDuplicateConstraintUsername) {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "username already exists",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		if strings.Contains(err.Error(), store_err.ErrDuplicateConstraintEmail) {
			echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error":   err.Error(),
				"message": "this email is already taken, try sign in?",
			})
			a.logger.Log(ctx, err).Send()
			return echoErr
		}

		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not persist the user",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	// in case of CI setup, no need to send verification emails
	if user.IsActive {
		echoErr := ctx.JSON(http.StatusCreated, echo.Map{
			"message": "user successfully created",
		})
		a.logger.Log(ctx, echoErr).Send()
		return echoErr
	}

	token, err := uuid.NewRandom()
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error creating random id for token",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr

	}
	err = a.emailStore.AddVerifyEmail(ctx.Request().Context(), token, user.ID)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "error adding verify email",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	webAppURL := a.c.WebAppConfig.GetAllowedURLFromEchoContext(ctx, a.c.Environment)
	err = a.emailClient.SendEmail(user, token.String(), email.VerifyEmailKind, webAppURL)
	if err != nil {
		echoErr := ctx.JSON(http.StatusInternalServerError, echo.Map{
			"error":   err.Error(),
			"message": "could not send verify link, please reach out to OpenRegistry Team",
		})
		a.logger.Log(ctx, err).Send()
		return echoErr
	}

	echoErr := ctx.JSON(http.StatusOK, echo.Map{
		"message": "signup was successful, please check your email to activate your account",
	})
	a.logger.Log(ctx, echoErr).Send()
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
