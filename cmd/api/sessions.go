package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"placeholder_project_tag/internal/data"

	"github.com/labstack/echo/v4"
)

func (app *application) createAndSetSessionCookie(c echo.Context, id *int64) error {
	ttl := app.config.Auth.SessionExpiration
	session, err := app.models.UserService.Sessions.New(c.Request().Context(), id, c.RealIP(), ttl)
	if err != nil {
		return err
	}

	c.SetCookie(&http.Cookie{
		Name:     data.TypeSession,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.Expiry,
	})

	return nil
}

func (app *application) getUserFromSession(ctx context.Context, session *data.Session) (*data.User, error) {
	if session.UserID == nil {
		return nil, errors.New("session has no attached user")
	}

	user, err := app.models.UserService.Users.GetByID(ctx, *session.UserID)
	if err != nil {
		return nil, fmt.Errorf("getUserFromSession: user lookup failed: %w", err)
	}

	// update last seen
	err = app.models.UserService.Sessions.UpdateLastSeen(ctx, session.ID)
	if err != nil {
		// non-fatal, log and continue
		app.logger.Error(err, map[string]any{
			"action":  "update last seen for session",
			"session": session,
		})
	}

	return user, nil
}
