package data

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type AdminService struct {
	Permissions *PermissionsModel
	Sessions    *SessionModel
	Users       *UserModel
}

func NewAdminService(appDB, monitorDB *sql.DB) *AdminService {
	return &AdminService{
		Permissions: &PermissionsModel{DB: appDB},
		Sessions:    &SessionModel{DB: appDB},
		Users:       &UserModel{DB: appDB},
	}
}

func (s *AdminService) GetAllUsers(ctx context.Context, filters UserFilters) ([]*User, *FilterMetadata, error) {
	// GROUP_CONCAT creates an array of values separated by ",", allowing all permissions for the user to be returned as part of one user row
	// query := `
	// 	SELECT
	// 		u.id,
	// 		u.created_at,
	// 		u.last_authenticated_at,
	// 		u.username,
	// 		GROUP_CONCAT(up.permission_code) AS permissions,
	// 		COUNT(*) OVER() AS total_count
	// 	FROM users u
	// 	LEFT JOIN users_permissions up ON up.user_id = u.id
	// 	GROUP BY u.id
	// 	ORDER BY u.created_at DESC
	// ;`

	var queryBuilder strings.Builder
	args := []any{}

	// queryBuilder.WriteString(`
	// 	SELECT logs.id, logs.level, logs.timestamp, logs.message, logs.details, logs.user_id,
	// 		(SELECT COUNT(*) FROM logs JOIN logs_fts ON logs.id = logs_fts.rowid WHERE 1=1
	// `)
	queryBuilder.WriteString(`
		SELECT
			u.id,
			u.created_at,
			u.last_authenticated_at,
			u.username,
			GROUP_CONCAT(up.permission_code) AS permissions,
			(SELECT COUNT(*) FROM users u2 JOIN user_usernames_fts fts2 ON u2.id = fts2.rowid WHERE 1=1
	`)

	getAllUsersFilterQueryHelper(&queryBuilder, &args, filters)

	queryBuilder.WriteString(") AS total_count FROM users u JOIN user_usernames_fts fts ON u.id = fts.rowid")

	getAllUsersFilterQueryHelper(&queryBuilder, &args, filters)

	queryBuilder.WriteString(fmt.Sprintf(" LEFT JOIN users_permissions up ON up.user_id = u.id WHERE 1=1 GROUP BY u.id ORDER BY %s %s, u.id DESC LIMIT ? OFFSET ?", filters.sortColumn(), filters.sortDirection()))
	args = append(args, filters.limit(), filters.offset())

	// log.Println("running GetAllUsers query")
	// log.Println(queryBuilder.String())

	rows, err := s.Users.DB.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, nil, ProcessSQLError(err, "error getting all users with filters")
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
			for p := range strings.SplitSeq(permString.String, ",") {
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

// Dynamically build filters for GetAllUsers query, based on filters provided
func getAllUsersFilterQueryHelper(q *strings.Builder, args *[]any, filters UserFilters) {
	if filters.UserName != "" {
		q.WriteString(" AND user_usernames_fts MATCH ?")
		*args = append(*args, filters.UserName)
	}
	if len(filters.UserID) > 0 {
		qp := fmt.Sprintf(" AND id IN (%s)", Placeholders(len(filters.UserID)))
		q.WriteString(qp)
		// q.WriteString(" AND level = ?")
		for _, val := range filters.UserID {
			*args = append(*args, val)
		}
	}
}
