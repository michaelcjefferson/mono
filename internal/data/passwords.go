package data

import (
	"errors"
	"placeholder_project_tag/pkg/validator"

	"golang.org/x/crypto/bcrypt"
)

// Using a pointer to plaintext allows differentiation between a password that hasn't been provided and a password that is an empty string, because nil value of a string is "" whereas nil value of a pointer is nil.
type password struct {
	plaintext *string
	hash      []byte
}

// Expose password hash for consumption by admin service - more secure than exporting password.Hash, which could lead to accidental logging etc.
func (p *password) Hash() []byte {
	return p.hash
}

// Convert plaintext password into []byte hash, and set both values on *password
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

// Compare provided plaintext password (provided by user) with hash (stored in db), return true for match
func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

// Ensure password exists, and is between 8 and 72 bytes long
func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 characters long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 characters long")
}
