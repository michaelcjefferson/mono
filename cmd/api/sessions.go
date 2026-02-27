package main

import (
	"context"
	"errors"
	"net/http"
	"placeholder_project_tag/internal/data"

	"github.com/labstack/echo/v4"
)

func (app *application) createAndSetSessionCookie(c echo.Context, id *int64) (*data.Session, error) {
	ttl := app.config.Auth.SessionExpiration
	session, err := app.models.UserService.Sessions.New(c.Request().Context(), id, c.RealIP(), ttl)
	if err != nil {
		return nil, err
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

	return session, nil
}

func (app *application) setSessionCookie(c echo.Context, session *data.Session) {
	c.SetCookie(&http.Cookie{
		Name:     data.TypeSession,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.Expiry,
	})
}

func (app *application) getUserFromSession(ctx context.Context, session *data.Session) (*data.User, error) {
	if session.UserID == nil {
		return nil, errors.New("session has no attached user")
	}

	user, err := app.models.UserService.GetUserBySession(ctx, session)
	if err != nil {
		return nil, err
	}

	return user, nil
}
