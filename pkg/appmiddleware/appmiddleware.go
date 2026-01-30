package appmiddleware

import (
	"fmt"

	"placeholder_project_tag/pkg/logging"

	"github.com/labstack/echo/v4"
)

func RecoverPanicMiddleware(next echo.HandlerFunc, logger *logging.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Deferring a function ensures it will execute as the stack is unwound in the case of a panic. It won't be used elsewhere, so an anonymous function works well.
		defer func() {
			// recover() is a built-in function that checks whether or not there has been a panic.
			if err := recover(); err != nil {
				// Set header which tells the server to close the connection after this has been sent.
				c.Response().Header().Set(echo.HeaderConnection, "close")
				logger.Error(err.(error), map[string]any{
					"OHNO": "couldn't recover from this error",
				})
				c.Error(fmt.Errorf("%v", err))
			}
		}()

		return next(c)
	}
}
