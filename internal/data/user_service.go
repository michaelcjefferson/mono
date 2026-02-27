package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUserAlreadyExists     = errors.New("tried to create a new entry for a user that already exists")
	ErrEmailAlreadyExists    = errors.New("a user with this email already exists")
	ErrUsernameAlreadyExists = errors.New("a user with this username already exists")
	ErrMissingUsername       = errors.New("username is undefined")
	ErrMissingPassword       = errors.New("password is undefined")
)

type UserService struct {
	Permissions *PermissionsModel
	Sessions    *SessionModel
	Tokens      *TokenModel
	Users       *UserModel
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{
		Permissions: &PermissionsModel{DB: db},
		Sessions:    &SessionModel{DB: db},
		Tokens:      &TokenModel{DB: db},
		Users:       &UserModel{DB: db},
	}
}

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

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	// As there is more than one query to perform, a transaction is required
	tx, err := s.Users.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = s.Users.Create(tx, ctx, user); err != nil {
		return err
	}

	fmt.Printf("user created: id is %v\n", user.ID)

	if user.GoogleID != "" {
		err = s.Users.UpdateGoogleID(tx, ctx, user)
		if err != nil {
			return err
		}
	}

	if err = s.Permissions.InsertManyForUserID(tx, ctx, user.Permissions, user.ID); err != nil {
		return err
	}

	err = tx.Commit()

	return err
}

func (s *UserService) ActivateUser(ctx context.Context, user *User) error {
	tx, err := s.Users.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = s.Users.Update(tx, ctx, user); err != nil {
		return err
	}

	if err = s.Permissions.InsertManyForUserID(tx, ctx, user.Permissions, user.ID); err != nil {
		return err
	}

	err = tx.Commit()

	return err
}

// TODO: consider skipping permissions look-up unless required - separate permissions look-up into requirePermission middleware?
// Return User struct matching the provided token, including the token's expiry and a slice of the user's permissions attached to their struct
func (s *UserService) GetUserByToken(ctx context.Context, tokenScope, tokenPlaintext string) (*User, string, error) {
	user, expiry, err := s.Users.GetForToken(ctx, tokenScope, tokenPlaintext)
	if err != nil {
		return nil, "", err
	}

	perms, err := s.Permissions.GetAllForUserID(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	user.Permissions = perms

	return user, expiry, nil
}

func (s *UserService) GetUserBySession(ctx context.Context, session *Session) (*User, error) {
	user, err := s.Users.GetByID(ctx, *session.UserID)
	if err != nil {
		return nil, err
	}

	perms, err := s.Permissions.GetAllForUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	user.Permissions = perms

	// update last seen
	err = s.Sessions.UpdateLastSeen(ctx, session.ID)
	if err != nil {
		// TODO: non-fatal, log and continue
		return nil, err
	}

	return user, nil
}
