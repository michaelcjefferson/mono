package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"placeholder_project_tag/internal/data"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// Type representing a JSON structure, for use in JSON responses
type envelope map[string]any

func (app *application) Render(c echo.Context, statusCode int, t templ.Component) error {
	c.Response().Writer.WriteHeader(statusCode)
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	return t.Render(c.Request().Context(), c.Response().Writer)
}

// Set the user for this request session to an anonymous user, and expire previously set cookies
func (app *application) resetUserToAnon(c echo.Context) {
	app.contextSetUser(c, data.AnonymousUser)
	c.SetCookie(&http.Cookie{
		Name:     data.TypeUserAuth,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Make dir with write permissions for the owner, and read and exec permissions for all others in group
func (app *application) createFileDirs() error {
	wd, _ := os.Getwd()
	log.Println("cwd:", wd)

	if err := os.MkdirAll("./db", 0755); err != nil {
		return fmt.Errorf("failed to create application data directory: %w", err)
	}
	// if err := os.MkdirAll("/data/placeholder_project_name", 0755); err != nil {
	// 	return fmt.Errorf("failed to create application data directory: %w", err)
	// }

	if app.config.Server.TLS.HTTPSOn {
		if err := os.MkdirAll("./tls", 0755); err != nil {
			return fmt.Errorf("failed to create application data directory: %w", err)
		}
	}

	return nil
}

// Take a function, and run it in a go routine with a deferred panic tidy-up.
func (app *application) background(fn func()) {
	// Use an application-level WaitGroup to keep track of all goroutines created by background(), by adding one to the WaitGroup counter before beginning the goroutine. When the app is shut down, use WaitGroup.Wait() to block until all background() goroutines have completed before terminating the app.
	app.wg.Add(1)

	go func() {
		// Decrement WaitGroup counter by 1 once fn() has returned.
		defer app.wg.Done()

		// Panics encountered in a goroutine will not be caught and tidied up by the recovery middleware. To remedy this, this deferred function will run after the fn() function below. By running recover(), it will catch any panics and log the error instead of terminating the application as would otherwise happen.
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error(fmt.Errorf("%s", err), nil)
			}
		}()

		fn()
	}()
}
