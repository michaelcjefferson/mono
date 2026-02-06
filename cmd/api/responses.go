package main

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func (app *application) redirectResponse(c echo.Context, path string, jsonStatus int, message any) error {
	// For API requests
	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		env := envelope{"message": message, "redirect": path}
		return c.JSON(jsonStatus, env)
		// For HTML page requests
	} else {
		return c.Redirect(http.StatusSeeOther, path)
	}
}
