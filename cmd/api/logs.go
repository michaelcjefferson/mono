package main

import (
	"errors"
	"net/http"
	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/apperrors"
	"placeholder_project_tag/pkg/validator"
	views "placeholder_project_tag/web/adminviews"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (app *application) getFilteredLogsPageHandler(c echo.Context) error {
	user := app.contextGetUser(c)

	filters := data.LogFilters{
		Filters: data.Filters{
			Page:         1,
			PageSize:     10,
			Sort:         "-timestamp",
			SortSafeList: []string{"level", "timestamp", "user_id", "-level", "-timestamp", "-user_id"},
		},
		Level:   []string{},
		Message: "",
		UserID:  []int{},
	}

	filters.Level = c.QueryParams()["level"]
	filters.Message = c.QueryParam("message")
	if uids := c.QueryParams()["user_id"]; len(uids) != 0 {
		for _, val := range uids {
			u, err := strconv.Atoi(val)
			if err != nil {
				return app.errorHTTPResponse(c, err, apperrors.ErrCodeBadRequest, map[string]any{
					"message": "malformed user id filters",
				})
			}
			filters.UserID = append(filters.UserID, u)
		}
	}

	// TODO: Refer to movies.go in greenlight to allow client-based sorting etc.
	if p := c.QueryParam("page"); p != "" {
		p, err := strconv.Atoi(p)
		if err != nil {
			return app.errorHTTPResponse(c, err, apperrors.ErrCodeBadRequest, map[string]any{
				"message": "malformed page number",
			})
		}
		filters.Page = p
	}

	if ps := c.QueryParam("page_size"); ps != "" {
		ps, err := strconv.Atoi(ps)
		if err != nil {
			return app.errorHTTPResponse(c, err, apperrors.ErrCodeBadRequest, map[string]any{
				"message": "malformed page size",
			})
		}
		filters.PageSize = ps
	}

	v := validator.New()
	if data.ValidateFilters(v, filters.Filters); !v.Valid() {
		return app.errorHTTPResponse(c, errors.New("filters malformed"), apperrors.ErrCodeFailedValidation, v.Errors)
	}

	logs, metadata, err := app.models.Logs.GetAll(c.Request().Context(), filters)
	if err != nil {
		return app.serverErrorResponse(c, err, nil)
	}

	return app.Render(c, http.StatusOK, views.LogsPage(logs, metadata, &filters, user, user.HasPermission(data.PermissionAdminAccess), true))
}

func (app *application) getIndividualLogPageHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	id, err := app.readIDParam(c)
	if err != nil {
		app.errorHTTPResponse(c, err, apperrors.ErrCodeBadRequest, nil)
		return err
	}

	log, err := app.models.Logs.GetForID(c.Request().Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.errorHTTPResponse(c, err, apperrors.ErrCodeResourceNotFound, nil)
		default:
			app.serverErrorResponse(c, err, nil)
		}
		return err
	}

	referer := c.Request().Header.Get("Referer")

	return app.Render(c, http.StatusOK, views.LogDetailPage(*log, referer, u, u.HasPermission(data.PermissionAdminAccess), true))
}

func (app *application) deleteIndividualLogHandler(c echo.Context) error {
	id, err := app.readIDParam(c)
	if err != nil {
		return app.errorAPIResponse(c, err, apperrors.ErrCodeBadRequest, nil)
	}

	err = app.models.Logs.DeleteForID(c.Request().Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			return app.errorAPIResponse(c, err, apperrors.ErrCodeResourceNotFound, nil)
		default:
			return app.serverErrorResponse(c, err, nil)
		}
	}

	return app.redirectResponse(c, "/admin/logs", http.StatusAccepted, "log successfully deleted")
}
