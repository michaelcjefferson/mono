package data

import (
	"context"
	"database/sql"
	"errors"
)

type Permission string

var (
	ErrInvalidPermission = errors.New("provided code is not a valid permission")
	ErrPermissionDenied  = errors.New("permission to access this resource is denied")
)

// User Permissions
const (
	PermissionAdminAccess Permission = "admin:access"
	PermissionAdminCreate Permission = "admin:create"
	PermissionUserAccess  Permission = "user:access"
)

// Ensure Permission string is one of the valid permissions defined by the app
func (p Permission) IsValid() bool {
	switch p {
	case PermissionAdminAccess, PermissionUserAccess:
		return true
	default:
		return false
	}
}

// Convert a slice of strings to a slice of Permissions, failing on an invalid permission
func StringsToPermissions(strs []string) ([]Permission, error) {
	permissions := make([]Permission, len(strs))
	for i, s := range strs {
		permissions[i] = Permission(s)
		if !permissions[i].IsValid() {
			return nil, ErrInvalidPermission
		}
	}
	return permissions, nil
}

type PermissionsModel struct {
	DB *sql.DB
}

// Get slice of Permissions for the provided ID number
func (m *PermissionsModel) GetAllForUserID(ctx context.Context, id int64) ([]Permission, error) {
	query := `
		SELECT permission_code
		FROM users_permissions
		WHERE user_id = $1;
	`

	args := []any{id}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// Make sure result from QueryContext is closed before returning from function
	defer rows.Close()

	var permissions []Permission

	for rows.Next() {
		var code string

		err := rows.Scan(
			&code,
		)
		if err != nil {
			return nil, err
		}

		if permission := Permission(code); !permission.IsValid() {
			return nil, ErrInvalidPermission
		}

		permissions = append(permissions, Permission(code))
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

// Check whether the provided user ID has the provided permission, returning boolean
func (m *PermissionsModel) GetOneForUserID(ctx context.Context, id int64, p Permission) (bool, error) {
	if !p.IsValid() {
		return false, nil
	}

	q := `
		SELECT EXISTS (
			SELECT 1
			FROM users_permissions
			WHERE user_id = $1 AND permission_code = $2
		);`

	var hasPermission bool

	args := []any{id, p}

	err := m.DB.QueryRowContext(ctx, q, args).Scan(&hasPermission)
	if err != nil {
		return false, err
	}

	return hasPermission, nil
}

// TODO: build dynamic query by looping over perms and adding (?, ?) to a []string for each, then run query once
func (m *PermissionsModel) InsertManyForUserID(tx *sql.Tx, ctx context.Context, perms []Permission, userID int64) error {
	permQuery, err := tx.PrepareContext(ctx, `
		INSERT INTO users_permissions (user_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING;`)
	if err != nil {
		return err
	}
	defer permQuery.Close()

	for _, p := range perms {
		if !p.IsValid() {
			return ErrInvalidPermission
		}

		args := []any{userID, string(p)}

		_, err = permQuery.ExecContext(ctx, args...)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				return ErrRecordNotFound
			default:
				return err
			}
		}
	}

	return nil
}
