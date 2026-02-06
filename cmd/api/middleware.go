package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"

	"github.com/labstack/echo/v4"
)

// Attach request ID to request
func (app *application) requestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			app.contextSetRequestID(c)

			return next(c)
		}
	}
}

// If a valid auth token is provided, set "user" value in request context to a struct containing the corresponding user's data. If an invalid token is provided, send an error.
func (app *application) authenticateUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		// TODO: Find a better place to declare this context - should be created when request is received. Context middleware?
		ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
		defer cancel()

		// TODO: Add a check for r.Context().Value("isAuthenticated").(bool) to prevent extra look-ups if user is already authenticated, and set the isAuthenticated value below once user has been found. Ensure that isAuthenticated doesn't lead to leaky security, where this can be parsed as true even if user has no or an expired token.

		// Get the http-only cookie containing the token from the request, and convert to a string
		cookie, err := c.Cookie(data.TypeUserAuth)

		// If the cookie can't be found, the user is not authenticated and should be set as an anonymous user
		if errors.Is(err, http.ErrNoCookie) {
			app.contextSetUser(c, data.AnonymousUser)
			return next(c)
		}

		token := cookie.Value

		v := validator.New()

		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			return app.errorHTTPResponse(c, nil, apperrors.ErrCodeInvalidToken, nil)
		}

		// Retrieve user data from user table based on the token provided.
		user, tokenExpiry, err := app.models.UserService.GetUserByToken(ctx, data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.resetUserToAnon(c)
				e := apperrors.NewAppError(apperrors.ErrCodeTokenExpired, nil, nil, app.contextGetRequestID(c))
				return app.redirectErrorResponse(c, "/sign-in", e)
			default:
				return app.serverErrorResponse(c, err, map[string]any{
					"action": "get user by token",
					"scope":  data.ScopeAuthentication,
				})
			}
		}

		// If token has only a short time before expiry, create a new token for that user
		expiryTime, err := time.Parse(time.RFC3339, tokenExpiry)
		if err != nil {
			return app.serverErrorResponse(c, err, map[string]any{
				"action":       "parse expiry time of token",
				"token expiry": tokenExpiry,
			})
		}

		expiryTimeFrame := time.Now().Add(app.config.Auth.JWTRefresh)

		// Check if the token expiry is within the timeframe, and if so, generate a new token and return it
		if expiryTime.Before(expiryTimeFrame) {
			app.logger.Info("token near expiry - creating new token and sending to user", map[string]any{
				"user id":           user.ID,
				"expiry time":       tokenExpiry,
				"expiry time frame": expiryTimeFrame,
			})
			app.createAndSetAuthTokenCookie(c, user.ID)
		}

		// Attach user data to context
		app.contextSetUser(c, user)

		// Call next handler in the chain.
		return next(c)
	}
}

// TODO: Add activation and requireActivatedUser for new user registrations after initial set-up - admins can go to an add user page, and enter an email address to send an activation code to. This creates an activation token in the database, and provides it as part of a link for the admin to copy and paste into an email to the new user. The new user can follow that link to be brought to an activation page, where they create a username and password, and a new account is created. Activation tokens valid for 24 (?) hours

// Runs after authenticate, only needed on protected routes - checks the context for the value of the user set by authenticate, and at this point only ensures that one exists, as it means that someone is logged in and can access protected routes
func (app *application) requireAuthenticatedUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := app.contextGetUser(c)

		// Prevent unauthenticated clients from accessing resource
		if user.IsAnonymous() {
			e := apperrors.NewAppError(apperrors.ErrCodeUnauthenticated, nil, nil, app.contextGetRequestID(c))
			return app.redirectErrorResponse(c, "/sign-in", e)
		}

		// TODO: consider removing, and relying only on requirePermissionCode middleware below (and only applying it to action routes - perhaps attach "allowed" property to user in db)
		// Prevent clients that don't have "user:access" permission from accessing resource
		if !user.HasPermission(data.PermissionUserAccess) {
			return app.errorHTTPResponse(c, nil, apperrors.ErrCodeResourceForbidden, nil)
		}

		return next(c)
	}
}

// Only allow access to a route if a user has the required permission
func (app *application) requirePermissionCode(permission data.Permission) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := app.contextGetUser(c)

			hasP := user.HasPermission(permission)

			app.logger.Info("requirePermissionCode hit", map[string]any{
				"user_id":         user.ID,
				"required_perm":   permission,
				"permissions":     user.Permissions,
				"has_perm_result": hasP,
			})

			if !user.HasPermission(permission) {
				return app.errorHTTPResponse(c, nil, apperrors.ErrCodeResourceForbidden, nil)
			}

			return next(c)
		}
	}
}

// Ensure that, if a panic is encountered, requests still receive a response indicating that there was an error, and the app doesn't shut down entirely
func (app *application) recoverPanicMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Deferring a function ensures it will execute as the stack is unwound in the case of a panic. It won't be used elsewhere, so an anonymous function works well.
		defer func() {
			// recover() is a built-in function that checks whether or not there has been a panic.
			if err := recover(); err != nil {
				// Set header which tells the server to close the connection after this has been sent.
				c.Response().Header().Set(echo.HeaderConnection, "close")
				app.logger.Error(err.(error), map[string]any{
					"OHNO": "couldn't recover from this error",
				})
				c.Error(fmt.Errorf("%v", err))
			}
		}()

		return next(c)
	}
}

// // TODO: Add to config for app, including instructions to find IP address of KAMAR instance
// func (app *application) processCORS(next http.Handler) http.Handler {
// 	c := cors.New(cors.Options{
// 		AllowedOrigins: []string{"https://localhost", "https://0.0.0.0"},
// 		// AllowedOrigins:   []string{"https://localhost", "https://10.100"},
// 		AllowCredentials: true,
// 		AllowedHeaders:   []string{"Origin", "Authorization", "Content-Type"},
// 		AllowedMethods:   []string{"GET", "POST"},
// 		// AllowedMethods:   []string{"POST"},
// 		Debug: true,
// 	})

// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		c.Handler(next).ServeHTTP(w, r)
// 	})
// }
