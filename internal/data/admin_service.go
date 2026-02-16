package data

import (
	"context"
	"database/sql"
	"strings"
)

type AdminService struct {
	Permissions *PermissionsModel
	Users       *UserModel
}

func NewAdminService(appDB, monitorDB *sql.DB) *AdminService {
	return &AdminService{
		Permissions: &PermissionsModel{DB: appDB},
		Users:       &UserModel{DB: appDB},
	}
}

func (s *AdminService) GetAllUsers(ctx context.Context, filters UserFilters) ([]*User, *FilterMetadata, error) {
	// GROUP_CONCAT creates an array of values separated by ",", allowing all permissions for the user to be returned as part of one user row
	query := `
		SELECT
			u.id,
			u.created_at,
			u.last_authenticated_at,
			u.username,
			GROUP_CONCAT(up.permission_code) AS permissions,
			COUNT(*) OVER() AS total_count
		FROM users u
		LEFT JOIN users_permissions up ON up.user_id = u.id
		GROUP BY u.id
		ORDER BY u.created_at DESC
	;`

	rows, err := s.Users.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	// Make sure result from QueryContext is closed before returning from function
	defer rows.Close()

	users := []*User{}
	totalUsers := 0

	for rows.Next() {
		var user User
		var permString sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.CreatedAt,
			&user.LastAuthenticatedAt,
			&user.UserName,
			&permString,
			&totalUsers,
		)
		if err != nil {
			return nil, nil, err
		}

		// Check to see if query returned a string of permissions, then split it by "," and convert each to Permission type before attaching to User
		if permString.Valid {
			permSlice := strings.Split(permString.String, ",")
			for _, p := range permSlice {
				perm := Permission(p)
				if perm.IsValid() {
					user.Permissions = append(user.Permissions, perm)
				}
			}
		}

		users = append(users, &user)
	}

	metadata := calculateMetadata(totalUsers, filters.Page, filters.PageSize)

	if err = rows.Err(); err != nil {
		return nil, nil, ProcessSQLError(err, "error getting all users from db")
	}

	return users, &metadata, nil
}
