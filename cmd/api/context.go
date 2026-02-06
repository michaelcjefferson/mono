package main

import (
	"context"

	"placeholder_project_tag/internal/data"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Contexts allow storing data in key/value pairs for the lifetime of a request. Creating an app-specific context key type, as below, prevents clashes with other libraries that may be storing data in context key-values - if, for example, another library also uses the "user" key as below, it will not cause any problems, because it is of a different type (though both have a base type of string)
type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	userContextKey contextKey = "user"
)

func (app *application) contextSetRequestID(c echo.Context) {
	requestID := uuid.New().String()

	// For http response header
	c.Response().Header().Set("X-Request-ID", requestID)

	// For echo handlers (contextGetRequestID())
	c.Set(string(requestIDKey), requestID)

	// For services that the context is passed to, eg. data.userModel.Create()
	ctx := context.WithValue(c.Request().Context(), requestIDKey, requestID)
	c.SetRequest(c.Request().WithContext(ctx))
}

func (app *application) contextGetRequestID(c echo.Context) string {
	id, ok := c.Get(string(requestIDKey)).(string)
	if !ok {
		id = "error retrieving request_id from context"
	}
	return id
}

// Add the provdied user struct to the request's context, using "user" as the key (with the type of userContextKey)
func (app *application) contextSetUser(c echo.Context, user *data.User) {
	c.Set(string(userContextKey), user)
}

// Seeing as contextGetUser will only be called when we firmly expect a user to exist already in the request's context, it is ok to throw a panic when one does not in fact exist, as it is a very unexpected situation.
func (app *application) contextGetUser(c echo.Context) *data.User {
	user, ok := c.Get(string(userContextKey)).(*data.User)

	if !ok {
		panic("could not get user from context")
	}

	return user
}
