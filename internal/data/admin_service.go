package data

import (
	"context"
	"database/sql"
	"errors"
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
	var query string
	args := []any{}

	if filters.UserName != "" {

		filterSQL, filterArgs := buildUserFilters(filters, true)

		query = fmt.Sprintf(`
			WITH filtered_users AS (
				SELECT u.id
				FROM user_usernames_fts
				JOIN users u ON u.id = user_usernames_fts.rowid
				WHERE 1=1 %s
			),
			paged_users AS (
				SELECT u.*
				FROM users u
				JOIN filtered_users fu ON fu.id = u.id
				ORDER BY %s %s, u.id DESC
				LIMIT ? OFFSET ?
			)
			SELECT
				pu.id,
				pu.created_at,
				pu.last_authenticated_at,
				pu.username,
				COALESCE(GROUP_CONCAT(up.permission_code), '') AS permissions,
				(SELECT COUNT(*) FROM filtered_users) AS total_count
			FROM paged_users pu
			LEFT JOIN users_permissions up ON up.user_id = pu.id
			GROUP BY pu.id
			ORDER BY %s %s, pu.id DESC
			`,
			filterSQL,
			filters.sortColumn(),
			filters.sortDirection(),
			filters.sortColumn(),
			filters.sortDirection(),
		)

		args = append(filterArgs, filters.limit(), filters.offset())
	} else {
		filterSQL, filterArgs := buildUserFilters(filters, false)

		query = fmt.Sprintf(`
			WITH filtered_users AS (
				SELECT u.id
				FROM users u
				WHERE 1=1 %s
			),
			paged_users AS (
				SELECT u.*
				FROM users u
				JOIN filtered_users fu ON fu.id = u.id
				ORDER BY %s %s, u.id DESC
				LIMIT ? OFFSET ?
			)
			SELECT
				pu.id,
				pu.created_at,
				pu.last_authenticated_at,
				pu.username,
				COALESCE(GROUP_CONCAT(up.permission_code), '') AS permissions,
				(SELECT COUNT(*) FROM filtered_users) AS total_count
			FROM paged_users pu
			LEFT JOIN users_permissions up ON up.user_id = pu.id
			GROUP BY pu.id
			ORDER BY %s %s, pu.id DESC
			`,
			filterSQL,
			filters.sortColumn(),
			filters.sortDirection(),
			filters.sortColumn(),
			filters.sortDirection(),
		)

		args = append(filterArgs, filters.limit(), filters.offset())
	}

	rows, err := s.Users.DB.QueryContext(ctx, query, args...)
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
func buildUserFilters(filters UserFilters, includeFTS bool) (string, []any) {
	var parts []string
	var args []any

	if includeFTS && filters.UserName != "" {
		parts = append(parts, "user_usernames_fts MATCH ?")
		// The addition of "*" will find any username that starts with the provided search, rather than looking for exact matches
		args = append(args, filters.UserName+"*")
	}

	// To match for any username containing the search string (not just starting with it), use the following:
	// OR u.username LIKE ?
	// ftsQuery := filters.UserName + "*"
	// likeQuery := "%" + filters.UserName + "%"

	// args = append(args, ftsQuery, likeQuery)

	if len(filters.UserID) > 0 {
		parts = append(parts,
			fmt.Sprintf("u.id IN (%s)", Placeholders(len(filters.UserID))),
		)
		for _, v := range filters.UserID {
			args = append(args, v)
		}
	}

	if len(parts) == 0 {
		return "", args
	}

	return " AND " + strings.Join(parts, " AND "), args
}

func (s *AdminService) UpdatePermsForUserID(ctx context.Context, permUpdate *PermissionUpdate) error {
	if permUpdate.UserID == nil {
		return errors.New("missing user id from permission update request")
	}

	tx, err := s.Users.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(permUpdate.RemovePerms) > 0 {
		err = s.Permissions.DeleteManyForUserID(tx, ctx, permUpdate.RemovePerms, *permUpdate.UserID)

		if err != nil {
			return ProcessSQLError(err, "failed to remove perms for user")
		}
	}

	if len(permUpdate.NewPerms) > 0 {
		err = s.Permissions.InsertManyForUserID(tx, ctx, permUpdate.NewPerms, *permUpdate.UserID)

		if err != nil {
			return ProcessSQLError(err, "failed to insert perms for user")
		}
	}

	err = tx.Commit()

	return err
}
