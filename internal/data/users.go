package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"placeholder_project_tag/pkg/validator"

	"github.com/google/uuid"
)

// For clients that have not provided an Authentication token as an Authorization header, allowing them to make user requests without being authenticated.
var AnonymousUser = &User{}

// Use the json:"-" tag to prevent these fields from appearing in any output when encoded to JSON.
type User struct {
	ID                  int64        `json:"id"`
	CreatedAt           time.Time    `json:"created_at"`
	UserName            string       `json:"username"`
	Email               string       `json:"email"`
	GoogleID            string       `json:"google_id"`
	Password            password     `json:"-"`
	Permissions         []Permission `json:"-"`
	Activated           bool         `json:"activated"`
	LastAuthenticatedAt time.Time    `json:"-"`
	Version             int          `json:"-"`
}

// Any user object can call this function which will return true if the user object doesn't have an email, password, activation, name, and ID associated with it.
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (u *User) HasPermission(p Permission) bool {
	return slices.Contains(u.Permissions, p)
}

// Ensure email exists and matches a standard email address pattern
func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

// Ensure username exists and is less than 500 bytes long
func ValidateUsername(v *validator.Validator, username string) {
	v.Check(username != "", "username", "must be provided")
	v.Check(len(username) >= 3, "username", "must be at least 3 characters long")
	v.Check(len(username) <= 50, "username", "must not be more than 50 characters long")
	v.Check(!validator.In(strings.ToLower(username), validator.ReservedUsernames...), "username", "invalid username")
	v.Check(validator.Matches(username, validator.UsernameRX), "username", "invalid username")
}

// Ensure user has a valid username, email, and password
func ValidateUser(v *validator.Validator, user *User) {
	ValidateUsername(v, user.UserName)

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	// If user's password hash is ever nil, it is an issue with our codebase rather than the user, so raise a panic rather than creating a validation error message.
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

type UserModel struct {
	DB *sql.DB
}

func (m *UserModel) Create(tx *sql.Tx, ctx context.Context, user *User) error {
	if user.UserName == "" {
		return ErrMissingUsername
	}
	if len(user.Password.hash) == 0 {
		return ErrMissingPassword
	}

	userQuery, err := tx.PrepareContext(ctx, `
		INSERT INTO users (uuid, username, email, password_hash, activated)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, version;`)
	if err != nil {
		return err
	}
	defer userQuery.Close()

	uuid := uuid.NewString()

	args := []any{uuid, user.UserName, user.Email, user.Password.hash, user.Activated}

	err = userQuery.QueryRowContext(ctx, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `UNIQUE constraint failed: users.username`:
			return ErrUserAlreadyExists
		case err.Error() == `UNIQUE constraint failed: users.email`:
			return ErrUserAlreadyExists
		default:
			return ProcessSQLError(err, "try to create user")
		}
	}

	return nil
}

// TODO: activate user, as they have registered with a Google account?
// Add/update user with attached user.GoogleID - primarily for use in user registration with a Google account
func (m *UserModel) UpdateGoogleID(tx *sql.Tx, ctx context.Context, user *User) error {
	// Should version be incremented, seeing as this will generally happen on user creation? Likely yes, as its purpose is to prevent double updates
	updateQuery, err := tx.PrepareContext(ctx, `
		UPDATE users
		SET google_id = $1, version = version + 1
		WHERE id = $2 AND version = $3
		RETURNING version
	`)
	if err != nil {
		return err
	}
	defer updateQuery.Close()

	args := []any{user.GoogleID, user.ID, user.Version}

	// Read new user version back into user struct
	err = updateQuery.QueryRowContext(ctx, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return ProcessSQLError(err, "try to update user's google id")
		}
	}

	return nil
}

// TODO: add filters eg. pagination
func (m *UserModel) GetAll(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, created_at, last_authenticated_at, username, email FROM users;
	`

	rows, err := m.DB.QueryContext(ctx, query)
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
			&user.Email,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, ProcessSQLError(err, "try to get all users")
	}

	return users, nil
}

func (m *UserModel) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `
		SELECT id, created_at, username, email, password_hash
		FROM users
		WHERE id = $1
	`

	var user User

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UserName,
		&user.Email,
		&user.Password.hash,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, ProcessSQLError(err, "try to get user by id")
		}
	}

	return &user, nil
}

// Retrieve user entry with matching username or email
func (m *UserModel) GetByUsernameOrEmail(ctx context.Context, identifier string) (*User, error) {
	query := `
		SELECT id, created_at, username, email, password_hash
		FROM users
		WHERE username = $1
		OR
		email = $1;
	`

	var user User

	err := m.DB.QueryRowContext(ctx, query, identifier).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UserName,
		&user.Email,
		&user.Password.hash,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, ProcessSQLError(err, "try to get user by username or email")
		}
	}

	return &user, nil
}

// Retrieve user entry with matching email address
func (m *UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, created_at, email, password_hash
		FROM users
		WHERE email = $1;
	`

	var user User

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Email,
		&user.Password.hash,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, ProcessSQLError(err, "try to get user by email")
		}
	}

	return &user, nil
}

// Retrieve user entry with matching tokenScope and tokenPlaintext
func (m *UserModel) GetForToken(ctx context.Context, tokenScope, tokenPlaintext string) (*User, string, error) {
	// This returns an array ([32]byte, specified length) rather than a slice ([]byte, unspecified length)
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	// INNER JOIN returns only the rows in the inner (overlapping) section of the Venn diagram created when users and tokens are joined on id. i.e., only rows with a matching id/user_id in both tables will exist in the join table.
	query := `
		SELECT users.id, users.created_at, users.username, users.email, users.google_id, users.password_hash, users.activated, users.version, tokens.expiry
		FROM users
		INNER JOIN tokens
		ON users.id = tokens.user_id
		WHERE tokens.hash = $1
		AND tokens.scope = $2
		AND tokens.expiry > datetime('now');`

	// Use [:] to convert the tokenHash [32]byte to a []byte. This is to match with SQLite's blob type, which tokens are stored as.
	args := []any{tokenHash[:], tokenScope}

	var user User
	var tokenExpiry string
	var googleID sql.NullString

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UserName,
		&user.Email,
		&googleID,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
		&tokenExpiry,
	)
	if googleID.Valid {
		user.GoogleID = googleID.String
	}

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, "", ErrRecordNotFound
		default:
			return nil, "", ProcessSQLError(err, "try to get user for token")
		}
	}

	return &user, tokenExpiry, nil
}

// Get number of users registered in database
func (m *UserModel) GetUserCount(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*) FROM users;
	`

	var count int

	err := m.DB.QueryRowContext(ctx, query).Scan(&count)

	return count, err
}

// Update db entry for user matching user.ID
func (m *UserModel) Update(tx *sql.Tx, ctx context.Context, user *User) error {
	query, err := tx.PrepareContext(ctx, `
		UPDATE users
		SET username = $1, email = $2, google_id = $3, password_hash = $4, activated = $5, version = version + 1
		WHERE id = $6 AND version = $7
		RETURNING version;
	`)

	args := []any{
		user.UserName,
		user.Email,
		user.GoogleID,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	err = query.QueryRowContext(ctx, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `UNIQUE constraint failed: users.username`:
			return ErrUserAlreadyExists
		case err.Error() == `UNIQUE constraint failed: users.email`:
			return ErrUserAlreadyExists
		// No rows means user version in update request doesn't match the user's current version in db - prevent race conditions (also feasibly means user id doesn't exist in db)
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return ProcessSQLError(err, "try to update user")
		}
	}

	return nil
}

// Update username for user matching user.ID
func (m *UserModel) UpdateUsername(ctx context.Context, user *User, username string) error {
	query := `
		UPDATE users
		SET username = $1, version = version + 1
		WHERE id = $2 AND version = $3
		RETURNING version;
	`

	args := []any{
		username,
		user.ID,
		user.Version,
	}

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `UNIQUE constraint failed: users.username`:
			return ErrUserAlreadyExists
		// No rows means user version in update request doesn't match the user's current version in db - prevent race conditions (also feasibly means user id doesn't exist in db)
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return ProcessSQLError(err, "try to update username for user")
		}
	}

	return nil
}

// Delete database entry for user matching userID
func (m *UserModel) Delete(ctx context.Context, userID int64) error {
	query := `
		DELETE FROM users WHERE id = $1;
	`

	_, err := m.DB.ExecContext(ctx, query, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}

	return nil
}
