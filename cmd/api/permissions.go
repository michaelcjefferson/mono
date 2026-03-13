package main

import (
	"fmt"
	"net/http"
	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"

	"github.com/labstack/echo/v4"
)

func (app *application) updateUserPermissionsHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	permUpdate := data.PermissionUpdate{}

	id, err := app.readIDParam(c)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}
	uid := int64(id)
	permUpdate.UserID = &uid

	err = c.Bind(&permUpdate)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}

	app.logger.Warn("attempting to update user permissions", map[string]any{
		"acting_user_id": u.ID,
		"perm_update":    permUpdate,
	})

	err = app.models.AdminService.UpdatePermsForUserID(c.Request().Context(), &permUpdate)
	if err != nil {
		return app.serverErrorResponse(c, err, nil)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": fmt.Sprintf("successfully updated permissions for user id %d", uid),
	})
}
