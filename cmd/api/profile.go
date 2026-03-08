package main

import (
	"fmt"
	"net/http"
	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"
	"placeholder_project_tag/web/views"

	"github.com/labstack/echo/v4"
)

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

// TODO
func (app *application) deleteAccountHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	app.logger.Warn(fmt.Sprintf("user %d attempted to delete their account", u.ID), nil)

	return c.JSON(http.StatusNotImplemented, map[string]any{
		"error":   "this endpoint is not yet implemented",
		"message": "this endpoint is not yet implemented",
	})
}
