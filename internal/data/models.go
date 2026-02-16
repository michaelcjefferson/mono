package data

import (
	"database/sql"
	"errors"
)

var (
	ErrDuplicateEntry = errors.New("cannot add to database - an item with these details already exists")
	ErrEditConflict   = errors.New("edit conflict")
	ErrRecordNotFound = errors.New("the requested record could not be found")
)

type Models struct {
	AdminService *AdminService
	Logs         LogModel
	UserService  *UserService
}

func NewModels(appDB, monitorDB *sql.DB) Models {
	return Models{
		AdminService: NewAdminService(appDB, monitorDB),
		Logs:         LogModel{DB: monitorDB},
		UserService:  NewUserService(appDB),
	}
}
