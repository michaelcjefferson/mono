package data

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *UserService) AdminExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM users_permissions
			WHERE permission_code = 'admin:access'
	);`

	var exists bool

	err := s.Permissions.DB.QueryRowContext(ctx, query).Scan(&exists)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return false, nil
		default:
			return false, ProcessSQLError(err, "try to find out whether admin user exists in db")
		}
	}

	return exists, nil
}
