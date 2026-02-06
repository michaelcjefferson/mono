package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// Redirect client to Google's auth flow, which will trigger a callback to /oauth/google/callback
func (app *application) googleLoginRedirectHandler(c echo.Context) error {
	u := app.contextGetUser(c)
	if !u.IsAnonymous() {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeSignOutRequired, nil)
	}

	// TODO: fill out AuthCodeURL params, find out how to use state
	url := app.googleAuth.AuthCodeURL("random_state")
	app.logger.Info("hit google login redirect handler", map[string]any{
		"url": url,
	})
	return app.redirectResponse(c, url, http.StatusAccepted, "you should be redirected")
}

// After user has authenticated with Google, use the returned code to obtain the user's Google profile, check db to see if user exists, and either register or sign in
func (app *application) googleCallbackHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 7*time.Second)
	defer cancel()

	// Convert code from Google's response to token that can be used to access user's Google data in corresponding request
	code := c.QueryParam("code")
	googleToken, err := app.googleAuth.Exchange(ctx, code)
	if err != nil {
		return app.errorHTTPResponse(c, err, apperrors.ErrCodeFailedValidation, nil)
	}

	user, err := app.getGoogleUserInfoWithAccessToken(c, ctx, googleToken)
	if err != nil {
		return app.serverErrorResponse(c, err, nil)
	}

	dbUser, err := app.googleUserLogInOrRegister(c, ctx, user)

	err = app.createAndSetAuthTokenCookie(c, dbUser.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action":      "create and set auth token cookie",
			"user":        dbUser,
			"google user": user,
		})
	}

	if len(dbUser.UserName) > 30 {
		return app.redirectResponse(c, "/profile/username", http.StatusAccepted, envelope{"user": dbUser})
	}

	return app.redirectResponse(c, "/", http.StatusAccepted, envelope{"user": dbUser})
}

func (app *application) googleMobileAuthHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 7*time.Second)
	defer cancel()

	var req struct {
		AccessToken string `json:"access_token"`
	}

	if err := c.Bind(&req); err != nil {
		return app.errorAPIResponse(c, nil, apperrors.ErrCodeBadRequest, nil)
	}

	googleToken := &oauth2.Token{
		AccessToken: req.AccessToken,
	}

	user, err := app.getGoogleUserInfoWithAccessToken(c, ctx, googleToken)
	if err != nil {
		return app.serverErrorResponse(c, err, nil)
	}

	// Check if user exists or create new user
	dbUser, err := app.googleUserLogInOrRegister(c, ctx, user)

	authToken, err := app.createAuthToken(ctx, dbUser.ID)
	if err != nil {
		return app.serverErrorResponse(c, err, map[string]any{
			"action":      "create and set auth token cookie",
			"user":        dbUser,
			"google user": user,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": authToken.Plaintext,
		"user":  dbUser,
	})
}

// Use access token from Google to get user email, Google ID, and other basic info Google shares about user
func (app *application) getGoogleUserInfoWithAccessToken(c echo.Context, ctx context.Context, token *oauth2.Token) (*data.GoogleUserInfo, error) {
	// Use token to get userinfo in separate request to Google
	client := app.googleAuth.Client(ctx, token)
	res, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, app.serverErrorResponse(c, err, map[string]any{
			"action":   "get userinfo from google client",
			"response": res,
			"error":    err,
		})
	}
	defer res.Body.Close()

	// Respond with error if Google token request failed
	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)

		return nil, app.errorHTTPResponse(c, fmt.Errorf("non-200 response from google: %s", string(bodyBytes)), apperrors.ErrCodeFailedValidation, nil)
	}

	// Read response body into GoogleUserInfo struct
	user := &data.GoogleUserInfo{}
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, app.serverErrorResponse(c, err, map[string]any{
			"action":        "decode google user info into struct",
			"response body": bodyBytes,
		})
	}

	return user, nil
}

// Using GoogleUserInfo, look up user in database - if user exists, return it, and if not, create it and then return it
func (app *application) googleUserLogInOrRegister(c echo.Context, ctx context.Context, user *data.GoogleUserInfo) (*data.User, error) {
	// Check for pre-existing user with the same email address in user db
	dbUser, err := app.models.UserService.Users.GetByEmail(ctx, user.Email)
	if err != nil {
		switch err {
		// If user doesn't already exist, register them
		case data.ErrRecordNotFound:
			// Prevent unverified Google accounts from being used
			if !user.VerifiedEmail {
				return nil, app.errorHTTPResponse(c, errors.New("your google account must be verified"), apperrors.ErrCodeBadRequest, nil)
			}

			dbUser := &data.User{
				Activated: true,
				Email:     user.Email,
				GoogleID:  user.ID,
				UserName:  uuid.New().String(),
				Password:  data.AnonymousUser.Password,
			}

			// If registered with Google account, user is considered activated, and gets all user permissions
			dbUser.Permissions = []data.Permission{data.PermissionUserAccess}

			v := validator.New()

			// ? Unnecessary, as no password is required and email and username are presumed valid due to uuid generation and prior google validation
			// if data.ValidateUser(v, input); !v.Valid() {
			// 	return app.errorAPIResponse(c, nil, apperrors.ErrCodeFailedValidation, v.Errors)
			// }

			err = app.models.UserService.CreateUser(ctx, dbUser)
			if err != nil {
				switch {
				case errors.Is(err, data.ErrUsernameAlreadyExists):
					v.AddError("username", "a user with this username already exists")
					return nil, app.errorAPIResponse(c, err, apperrors.ErrCodeUsernameAlreadyExists, v.Errors)
				case errors.Is(err, data.ErrEmailAlreadyExists):
					v.AddError("email", "a user with this email already exists")
					return nil, app.errorAPIResponse(c, err, apperrors.ErrCodeEmailAlreadyExists, v.Errors)
				default:
					return nil, app.serverErrorResponse(c, err, map[string]any{
						"action": "create user",
						"user":   dbUser,
					})
				}
			}

			app.logger.Info("new user registered", map[string]any{
				"user_id":    dbUser.ID,
				"created at": dbUser.CreatedAt,
				"username":   dbUser.UserName,
			})

			return dbUser, nil
		// In case of other error, respond with error
		default:
			return nil, app.serverErrorResponse(c, err, map[string]any{
				"action": "get user by email, based on google auth",
				"user":   user,
			})
		}
	}

	// If the user's account hasn't been activated due to previously only signing in with username and password (and not activating via email), activate them, as a successful Google sign in counts as activation
	if dbUser.GoogleID == "" {
		dbUser.GoogleID = user.ID
		dbUser.Activated = true
		dbUser.Permissions = []data.Permission{data.PermissionUserAccess}

		err = app.models.UserService.ActivateUser(ctx, dbUser)
		if err != nil {
			return nil, err
		}
	}

	app.logger.Info("user logged in", map[string]any{
		"user_id": dbUser.ID,
		"method":  "google",
	})

	return dbUser, nil
}
