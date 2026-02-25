package main

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"
	"placeholder_project_tag/web/views"
)

type userInput struct {
	UserName        string `json:"username"`
	Email           string `json:"email"`
	UsernameOrEmail string `json:"usernameOrEmail"`
	Password        string `json:"password"`
}

type PasswordUpdateInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (app *application) registerPageHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	if !user.IsAnonymous() {
		e := apperrors.NewAppError(apperrors.ErrCodeSignOutRequired, nil, nil, app.contextGetRequestID(c))
		return app.redirectErrorResponse(c, "/", e)
	}

	return app.Render(c, http.StatusAccepted, views.AuthPage("register"))
}

func (app *application) registerUserHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	if !u.IsAnonymous() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeSignOutRequired, nil)
	}

	input := userInput{}

	err := c.Bind(&input)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}

	user := &data.User{
		UserName: input.UserName,
		Email:    input.Email,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "set password - convert plaintext to byte hash",
			"user":   user,
		})
	}

	// User is not activated when registering in this way - only allow user access, and add other permissions (if any) on activation
	user.Permissions = []data.Permission{data.PermissionUserAccess}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeFailedValidation, v.Errors)
	}

	err = app.models.UserService.CreateUser(c.Request().Context(), user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrUsernameAlreadyExists):
			v.AddError("username", "a user with this username already exists")
			return app.errorAPIResponse(c, err, apperrors.ErrCodeUsernameAlreadyExists, v.Errors)
		case errors.Is(err, data.ErrEmailAlreadyExists):
			v.AddError("email", "a user with this email already exists")
			return app.errorAPIResponse(c, err, apperrors.ErrCodeEmailAlreadyExists, v.Errors)
		default:
			return app.serverErrorResponse(c, err, map[string]any{
				"action": "create user",
				"user":   user,
			})
		}
	}

	err = app.createAndSetAuthTokenCookie(c, user.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "create and set auth token cookie",
			"user":   user,
		})
	}

	err = app.createAndSetSessionCookie(c, &user.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "create and set session cookie",
			"user":   user,
		})
	}

	app.logger.Info("new user registered", map[string]any{
		"user_id":    user.ID,
		"created at": user.CreatedAt,
		"username":   user.UserName,
		"platform":   "web",
	})

	// return c.JSON(http.StatusAccepted, envelope{"authenticated": true})
	return app.redirectResponse(c, "/", http.StatusAccepted, envelope{"user": user, "authenticated": true})
}

// Prevent user from accessing sign in page and handler if they are already logged in
func (app *application) signInPageHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	if !user.IsAnonymous() {
		e := apperrors.NewAppError(apperrors.ErrCodeSignOutRequired, nil, nil, app.contextGetRequestID(c))
		return app.redirectErrorResponse(c, "/", e)
	}

	return app.Render(c, http.StatusAccepted, views.AuthPage("sign-in"))
}

func (app *application) signInHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	if !u.IsAnonymous() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeSignOutRequired, nil)
	}

	input := userInput{}

	err := c.Bind(&input)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}

	v := validator.New()

	// data.ValidateUsername(v, input.UserName)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeFailedValidation, v.Errors)
	}

	// Retrieve row from users table that matches the provided username
	user, err := app.models.UserService.Users.GetByUsernameOrEmail(c.Request().Context(), input.UsernameOrEmail)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			return app.errorAPIResponse(c, err, apperrors.ErrCodeInvalidCredentials, nil)
		default:
			return app.serverErrorResponse(c, err, map[string]any{
				"action": "get user by username or email",
				"input":  input,
			})
		}
	}

	// Compare plaintext password provided by the client with hashed password from row retrieved from user table (SECURITY RISK - MAN-IN-MIDDLE/FAKE WEBSITE ATTACK??)
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "check the user password matches stored hash",
			"user":   user,
		})
	}

	if !match {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeInvalidCredentials, nil)
	}

	err = app.createAndSetAuthTokenCookie(c, user.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "create and set auth token cookie",
			"user":   user,
		})
	}

	err = app.createAndSetSessionCookie(c, &user.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "create and set session cookie",
			"user":   user,
		})
	}

	app.logger.Info("user logged in", map[string]any{
		"user_id":  user.ID,
		"method":   "app-based",
		"platform": "web",
	})

	return app.redirectResponse(c, "/", http.StatusAccepted, envelope{"user": user})
}

func (app *application) logoutUserHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	if user.IsAnonymous() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeUnauthenticated, nil)
	}

	deleted, err := app.models.UserService.Tokens.DeleteAllForUser(user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			return app.errorAPIResponse(c, err, apperrors.ErrCodeResourceNotFound, nil)
		default:
			return app.serverErrorResponse(c, err, map[string]any{
				"action": "delete all tokens for user",
				"user":   user,
			})
		}
	}

	sessionCookie, _ := c.Cookie(data.TypeSession)
	err = app.models.UserService.Sessions.Delete(c.Request().Context(), sessionCookie.Value)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			return app.errorAPIResponse(c, err, apperrors.ErrCodeResourceNotFound, nil)
		default:
			return app.serverErrorResponse(c, err, map[string]any{
				"action": "delete all sessions for user",
				"user":   user,
			})
		}
	}

	app.logger.Info("user logged out", map[string]any{
		"user_id":        user.ID,
		"tokens_deleted": deleted,
	})

	app.resetUserToAnon(c)

	return app.redirectResponse(c, "/sign-in", http.StatusAccepted, "successfully logged out")
}

func (app *application) usernamePageHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	return app.Render(c, http.StatusAccepted, views.UsernamePage(user))
}

func (app *application) usernameUpdateHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	if user.IsAnonymous() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeUnauthenticated, nil)
	}

	userInput := userInput{}

	err := c.Bind(&userInput)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}

	v := validator.New()

	data.ValidateUsername(v, userInput.UserName)

	if !v.Valid() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeFailedValidation, v.Errors)
	}

	app.logger.Info("user attempting to update username", map[string]any{
		"user_id":      user.ID,
		"old_username": user.UserName,
		"new_username": userInput.UserName,
	})

	err = app.models.UserService.Users.UpdateUsername(c.Request().Context(), user, userInput.UserName)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeUsernameAlreadyExists, nil)
	}

	return app.redirectResponse(c, "/", http.StatusAccepted, "successfully set username")
}
