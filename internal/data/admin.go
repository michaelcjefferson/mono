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

func (s *UserService) GetAllUsers(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, created_at, last_authenticated_at, username FROM users;
	`

	rows, err := s.Users.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// Make sure result from QueryContext is closed before returning from function
	defer rows.Close()

	users := []*User{}

	for rows.Next() {
		var user User

		err := rows.Scan(
			&user.ID,
			&user.CreatedAt,
			&user.LastAuthenticatedAt,
			&user.UserName,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, ProcessSQLError(err, "error getting all users from db")
	}

	return users, nil
}
