package main

import (
	"context"
	"errors"
	"net/http"
	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"
	"placeholder_project_tag/web/adminviews"
	"time"

	"github.com/labstack/echo/v4"
)

func (app *application) initialiseAdmin(c echo.Context) error {
	adminExists, err := app.models.UserService.AdminExists()
	if err != nil {
		app.logger.Fatal(err, map[string]any{
			"action": "check whether admin exists in db",
		})
	}
	if adminExists {
		return echo.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()

	var input struct {
		InitKey  string `json:"init_key"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.Bind(&input); err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, map[string]any{
			"message": "request was malformed - follow the format provided in logs/email you received",
		})
	}

	user := &data.User{
		UserName:  "admin123",
		Email:     input.Email,
		Activated: true,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action": "set password - convert plaintext to byte hash",
			"user":   user,
		})
	}

	user.Permissions = []data.Permission{data.PermissionUserAccess, data.PermissionAdminAccess, data.PermissionAdminCreate}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeFailedValidation, v.Errors)
	}

	err = app.models.UserService.CreateUser(ctx, user)
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

	app.logger.Info("new admin user registered", map[string]any{
		"user_id":    user.ID,
		"created at": user.CreatedAt,
		"username":   user.UserName,
		"platform":   "web",
	})

	return c.JSON(http.StatusAccepted, envelope{"authenticated": true, "message": "navigate to site in the browser and log in\n\nuesrname is 'admin123'"})
	// return app.redirectResponse(c, "/", http.StatusAccepted, envelope{"user": user, "authenticated": true})
}

func (app *application) adminDashboardHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	app.logger.Info("user attempted to access admin dashboard", map[string]any{
		"user": u,
	})

	hasPerm := u.HasPermission(data.PermissionAdminAccess)

	if !hasPerm {
		return echo.ErrNotFound
	}

	return app.Render(c, http.StatusAccepted, adminviews.Dashboard(u, hasPerm))
}
