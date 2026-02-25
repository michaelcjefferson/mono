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

// Attach 3 second timeout to each request context - use c.Request().Context() in any function that requires context
func (app *application) requestContextMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
		defer cancel()
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}

// TODO: incorporate below
// func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
//     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//         // 1. try the existing auth token first (your current logic)
//         authToken := extractAuthToken(r)
//         if authToken != "" {
//             claims, err := validateAuthToken(authToken)
//             if err == nil {
//                 // still valid — attach to context and continue as normal
//                 ctx := attachClaimsToContext(r.Context(), claims)
//                 next.ServeHTTP(w, r.WithContext(ctx))
//                 return
//             }
//             // token is invalid or expired — fall through to session check
//         }

//         // 2. auth token missing or expired — check session
//         cookie, err := r.Cookie("session")
//         if err != nil {
//             http.Error(w, "unauthorised", http.StatusUnauthorized)
//             return
//         }

//         session, err := h.sessionStore.Get(cookie.Value)
//         if err != nil || session == nil || time.Now().After(session.ExpiresAt) {
//             http.Error(w, "unauthorised", http.StatusUnauthorized)
//             return
//         }

//         // 3. valid session — load user, issue a fresh auth token
//         user, err := h.userStore.GetByID(session.UserID)
//         if err != nil {
//             http.Error(w, "internal error", http.StatusInternalServerError)
//             return
//         }

//         freshToken, err := issueAuthToken(user) // your existing token generation
//         if err != nil {
//             http.Error(w, "internal error", http.StatusInternalServerError)
//             return
//         }

//         // return the new auth token to the client however you currently do it
//         // e.g. in a header so the client can store and reuse it
//         w.Header().Set("X-Refresh-Token", freshToken)

//         // update last seen
//         h.sessionStore.UpdateLastSeen(session.ID)

//         // attach claims to context as normal
//         claims, _ := validateAuthToken(freshToken)
//         ctx := attachClaimsToContext(r.Context(), claims)
//         next.ServeHTTP(w, r.WithContext(ctx))
//     })
// }

// If a valid auth token is provided, set "user" value in request context to a struct containing the corresponding user's data. If an invalid token is provided, send an error.
// func (app *application) authenticateUser(next echo.HandlerFunc) echo.HandlerFunc {
// 	return func(c echo.Context) error {

// 		// Get the http-only cookie containing the token from the request, and convert to a string
// 		authCookie, err := c.Cookie(data.TypeUserAuth)

// 		// If the cookie can't be found, check for session token, and if present and valid, set new auth token. If that also doesn't exist, the user is not authenticated and should be set as an anonymous user
// 		if errors.Is(err, http.ErrNoCookie) {
// 			sessCookie, err := c.Cookie(data.TypeSession)
// 			if errors.Is(err, http.ErrNoCookie) {
// 				app.contextSetUser(c, data.AnonymousUser)
// 				return next(c)
// 			}

// 			session, err := app.models.UserService.Sessions.Get(c.Request().Context(), sessCookie.Value)
// 			if err != nil || session == nil || time.Now().After(session.Expiry) {
// 				app.contextSetUser(c, data.AnonymousUser)
// 				return next(c)
// 			}
// 		}

// 		authToken := authCookie.Value

// 		v := validator.New()

// 		if data.ValidateTokenPlaintext(v, authToken); !v.Valid() {
// 			return app.errorHTTPResponse(c, nil, apperrors.ErrCodeInvalidToken, nil)
// 		}

// 		// Retrieve user data from user table based on the token provided.
// 		user, tokenExpiry, err := app.models.UserService.GetUserByToken(c.Request().Context(), data.ScopeAuthentication, authToken)
// 		if err != nil {
// 			switch {
// 			case errors.Is(err, data.ErrRecordNotFound):
// 				app.resetUserToAnon(c)
// 				e := apperrors.NewAppError(apperrors.ErrCodeTokenExpired, nil, nil, app.contextGetRequestID(c))
// 				return app.redirectErrorResponse(c, "/sign-in", e)
// 			default:
// 				return app.serverErrorResponse(c, err, map[string]any{
// 					"action": "get user by token",
// 					"scope":  data.ScopeAuthentication,
// 				})
// 			}
// 		}

// 		// If token has only a short time before expiry, create a new token for that user
// 		expiryTime, err := time.Parse(time.RFC3339, tokenExpiry)
// 		if err != nil {
// 			return app.serverErrorResponse(c, err, map[string]any{
// 				"action":       "parse expiry time of token",
// 				"token expiry": tokenExpiry,
// 			})
// 		}

// 		expiryTimeFrame := time.Now().Add(app.config.Auth.JWTRefresh)

// 		// Check if the token expiry is within the timeframe, and if so, generate a new token and return it
// 		if expiryTime.Before(expiryTimeFrame) {
// 			app.logger.Info("token near expiry - creating new token and sending to user", map[string]any{
// 				"user id":           user.ID,
// 				"expiry time":       tokenExpiry,
// 				"expiry time frame": expiryTimeFrame,
// 			})
// 			app.createAndSetAuthTokenCookie(c, user.ID)
// 		}

// 		// Attach user data to context
// 		app.contextSetUser(c, user)

// 		// Call next handler in the chain.
// 		return next(c)
// 	}
// }

func (app *application) authenticateUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Ensure all users (including anonymous) have a session associated with them
		session, err := app.getOrCreateSession(c)
		if err != nil {
			return app.serverErrorResponse(c, err, nil)
		}

		// Try to resolve an authenticated user from auth token or session
		user, err := app.resolveUser(c, session)
		if err != nil {
			app.resetUserToAnon(c)
			switch {
			case errors.Is(err, data.ErrInvalidToken):
				return app.errorHTTPResponse(c, err, apperrors.ErrCodeInvalidToken, nil)
			case errors.Is(err, data.ErrTokenExpired):
				e := apperrors.NewAppError(apperrors.ErrCodeTokenExpired, nil, nil, app.contextGetRequestID(c))
				return app.redirectErrorResponse(c, "/sign-in", e)
			case errors.Is(err, data.ErrRecordNotFound):
				e := apperrors.NewAppError(apperrors.ErrCodeResourceNotFound, nil, nil, app.contextGetRequestID(c))
				return app.redirectErrorResponse(c, "/sign-in", e)
			default:
				return app.serverErrorResponse(c, err, nil)
			}
		}

		// Attach user (authenticated or anonymous) and session to context
		app.contextSetUser(c, user)
		app.contextSetSession(c, session)

		return next(c)
	}
}

// Always returns a data.Session - existing or new anonymous one
func (app *application) getOrCreateSession(c echo.Context) (*data.Session, error) {
	cookie, err := c.Cookie(data.TypeSession)

	if err == nil {
		session, err := app.models.UserService.Sessions.Get(c.Request().Context(), cookie.Value)

		if err == nil && session != nil && time.Now().Before(session.Expiry) {
			app.models.UserService.Sessions.UpdateLastSeen(c.Request().Context(), session.ID)

			return session, nil
		}
		// session invalid/expired - create new one
	}

	session, err := app.models.UserService.Sessions.New(c.Request().Context(), nil, c.RealIP(), app.config.Auth.SessionExpiration)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Try to attach user via auth token, fall back to auth token refresh if session token is attached to user, fall back to anonymous user
func (app *application) resolveUser(c echo.Context, session *data.Session) (*data.User, error) {
	ctx := c.Request().Context()

	authCookie, err := c.Cookie(data.TypeUserAuth)
	if err == nil {
		authToken := authCookie.Value

		v := validator.New()

		if data.ValidateTokenPlaintext(v, authToken); !v.Valid() {
			return data.AnonymousUser, data.ErrInvalidToken
		}

		// Retrieve user data from user table based on the token provided.
		user, tokenExpiry, err := app.models.UserService.GetUserByToken(ctx, data.ScopeAuthentication, authToken)
		if err != nil {
			return data.AnonymousUser, err
		}

		// If token has only a short time before expiry, create a new token for that user
		expiryTime, err := time.Parse(time.RFC3339, tokenExpiry)
		if err != nil {
			return data.AnonymousUser, err
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

		// If session doesn't yet have a user id attached, attach it
		if session.UserID == nil {
			err = app.models.UserService.Sessions.AttachUser(ctx, session.ID, user.ID)
			if err != nil {
				return data.AnonymousUser, err
			}
			session.UserID = &user.ID
			// If there is a mismatch between session user id and user id, create a new session and replace old one
		} else if *session.UserID != user.ID {
			app.logger.Warn("mismatch between session user id and user id", map[string]any{
				"session": session,
				"user":    user,
			})

			newSession, err := app.models.UserService.Sessions.New(c.Request().Context(), &user.ID, c.RealIP(), app.config.Auth.SessionExpiration)
			if err != nil {
				return data.AnonymousUser, err
			}

			*session = *newSession
		}

		return user, nil
	}

	if session.UserID != nil {
		// Attempt to get user from session user id
		user, err := app.getUserFromSession(ctx, session)
		if err == nil {
			err = app.createAndSetAuthTokenCookie(c, user.ID)
			if err != nil {
				app.logger.Error(err, map[string]any{
					"action":  "create and set new auth token cookie from session",
					"session": session,
					"user":    user,
				})
			}
			return user, err
		}
		app.logger.Error(err, map[string]any{
			"action":  "look up user from session - resolveUser middleware",
			"session": session,
		})
	}

	// Fall back to anonymous user
	return data.AnonymousUser, nil
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

			// hasP := user.HasPermission(permission)

			// app.logger.Info("requirePermissionCode hit", map[string]any{
			// 	"user_id":         user.ID,
			// 	"required_perm":   permission,
			// 	"permissions":     user.Permissions,
			// 	"has_perm_result": hasP,
			// })

			if !user.HasPermission(permission) {
				app.logger.Warn("unauthorised access attempt", map[string]any{
					"user_id":             user.ID,
					"email":               user.Email,
					"path":                c.Request().URL.Path,
					"ip":                  c.Request().RemoteAddr,
					"required_permission": permission,
					"has_permissions":     user.Permissions,
				})

				// Return 404 for unpermitted admin:access routes so that they are obscured from hackers
				if permission == data.PermissionAdminAccess {
					return echo.NewHTTPError(http.StatusNotFound)
				} else {
					return app.errorHTTPResponse(c, nil, apperrors.ErrCodeResourceForbidden, nil)
				}
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
