package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (app *application) healthcheckHandler(c echo.Context) error {
	app.logger.Info("hit api healthcheck handler", nil)

	return c.JSON(http.StatusOK, map[string]any{
		"status": "available",
		"system_info": map[string]string{
			"project":     app.config.Project.Name,
			"environment": app.config.Project.Env,
			"service":     "api service",
		},
	})
}
