package main

import (
	"errors"
	"net/http"
	"strings"

	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/web/views"

	"github.com/labstack/echo/v4"
)

func (app *application) logAppError(c echo.Context, err apperrors.AppError) {
	app.logger.Warn("error: client request failed", map[string]any{
		"app_error":      err,
		"request_ip":     c.RealIP(),
		"request_method": c.Request().Method,
		"request_url":    c.Request().URL.String(),
		"request_id":     err.RequestID,
	})
}

func (app *application) logRequest(c echo.Context, message string) {
	app.logger.Debug(message, map[string]any{
		"request_ip":     c.RealIP(),
		"request_method": c.Request().Method,
		"request_url":    c.Request().URL.String(),
		"request_id":     app.contextGetRequestID(c),
	})
}

func (app *application) errorHTTPResponse(c echo.Context, err error, code apperrors.ErrorCode, details map[string]any) error {
	appErr := apperrors.NewAppError(code, err, details, app.contextGetRequestID(c))

	app.logAppError(c, appErr)

	e := appErr.ToHTTPError()

	return app.Render(c, e.StatusCode, views.ErrorPage(e))
}

func (app *application) errorAPIResponse(c echo.Context, err error, code apperrors.ErrorCode, details map[string]any) error {
	appErr := apperrors.NewAppError(code, err, details, app.contextGetRequestID(c))

	app.logAppError(c, appErr)

	return apperrors.SendAPIErrorResponse(c, appErr, "")
}

// If the client accepts JSON (which will be the case when using fetch() in the browser for API calls, CURL etc.), provide a JSON response including a status code, redirect path to follow if desired by the client, and error message - otherwise respond with an http.Redirect. http.Redirect will occur if a client tries to access a page via its URL - it is a GET request and doesn't include the Accepts JSON header
func (app *application) redirectErrorResponse(c echo.Context, path string, e apperrors.AppError) error {
	app.logAppError(c, e)

	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		return apperrors.SendAPIErrorResponse(c, e, path)
	} else {
		return c.Redirect(http.StatusSeeOther, path)
	}
}

func (app *application) serverErrorResponse(c echo.Context, err error, details map[string]any) error {
	app.logger.Error(err, map[string]any{
		"request_ip":     c.RealIP(),
		"request_method": c.Request().Method,
		"request_url":    c.Request().URL.String(),
		"request_id":     app.contextGetRequestID(c),
		"details":        details,
	})

	return app.errorHTTPResponse(c, errors.New("the server could not process this request"), apperrors.ErrCodeInternalServer, nil)
}

func (app *application) errorCheckHandler(c echo.Context) error {
	return app.errorHTTPResponse(c, nil, apperrors.ErrCodeFileTooBig, envelope{"file_size": "15 million kilobytes", "max_file_size": "3 kilobytes"})
}
