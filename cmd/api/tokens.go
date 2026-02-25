package main

import (
	"context"
	"net/http"
	"time"

	"placeholder_project_tag/internal/data"

	"github.com/labstack/echo/v4"
)

func (app *application) createAndSetAuthTokenCookie(c echo.Context, id int64) error {
	ttl := app.config.Auth.JWTExpiration
	token, err := app.models.UserService.Tokens.New(c.Request().Context(), id, ttl, data.ScopeAuthentication)
	if err != nil {
		return err
	}

	c.SetCookie(&http.Cookie{
		Name:     data.TypeUserAuth,
		Value:    token.Plaintext,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
	})

	return nil
}

func (app *application) createAuthToken(ctx context.Context, id int64) (*data.Token, error) {
	ttl := app.config.Auth.JWTExpiration
	token, err := app.models.UserService.Tokens.New(ctx, id, ttl, data.ScopeAuthentication)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (app *application) initiateTokenDeletionCycle() {
	app.background(func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				tokensDeleted, err := app.models.UserService.Tokens.DeleteExpiredTokens()
				if err != nil {
					app.logger.Error(err, nil)
				}
				app.logger.Info("purged expired tokens", map[string]any{
					"tokensDeleted": tokensDeleted,
				})
				sessionsDeleted, err := app.models.UserService.Sessions.DeleteExpiredSessions()
				if err != nil {
					app.logger.Error(err, nil)
				}
				app.logger.Info("purged expired sessions", map[string]any{
					"sessionsDeleted": sessionsDeleted,
				})
			case <-app.isShuttingDown:
				app.logger.Info("token deletion cycle ending - shut down signal received", nil)
				return
			}
		}
	})
}
