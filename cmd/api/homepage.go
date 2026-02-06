package main

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"placeholder_project_tag/web/views"
)

func (app *application) homepagePageHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	return app.Render(c, http.StatusOK, views.HomePage(u))
}
