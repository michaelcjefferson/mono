package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"

	"placeholder_project_tag/pkg/validator"
)

// Token types
const (
	TypeAdminAuth = "admin_auth_token"
	TypeUserAuth  = "user_auth_token"
)

// Token scopes
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

// JSON tags dictate which fields will be encoded into JSON for the client, and the names  of their corresponding keys ("token" is more meaningful for the client than "plaintext")
type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

// ttl (time-to-live) is added to time.Now to create a token expiry
func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl).UTC(),
		Scope:  scope,
	}

	// crypto/rand.Read fills a byte slice with random bytes from CSPRNG. This token will have an entropy (randomness) of 16 bytes. Base32 encoding means the plaintext token itself will be 26 bytes long.
	randomBytes := make([]byte, 16)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// Omit the possible = sign at the end by using WithPadding(base32.NoPadding)
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(token.Plaintext))
	// Convert the array returned by Sum256() to a slice using [:], so that it's easier to work with.
	token.Hash = hash[:]

	return token, nil
}

// Ensure that plaintext token exists and is 26 bytes long
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

type TokenModel struct {
	DB *sql.DB
}

// Whenever a token is created, the next step will be for it to be stored in the tokens table on the database. So, call m.Insert() as part of the token creation process.
func (m *TokenModel) New(ctx context.Context, userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(ctx, token)
	return token, err
}

// Insert token into tokens table of app db
func (m *TokenModel) Insert(ctx context.Context, token *Token) error {
	// As there is more than one query to perform, a transaction is required
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertQuery, err := tx.PrepareContext(ctx, `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES ($1, $2, $3, $4);`)

	if err != nil {
		return err
	}
	defer insertQuery.Close()

	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	_, err = insertQuery.ExecContext(ctx, args...)
	if err != nil {
		return err
	}

	updateQuery, err := tx.PrepareContext(ctx, `
		UPDATE users SET last_authenticated_at = $1 WHERE id = $2;`)

	if err != nil {
		return err
	}
	defer updateQuery.Close()

	now := time.Now().UTC()

	args = []any{now, token.UserID}
	_, err = updateQuery.ExecContext(ctx, args...)
	if err != nil {
		return err
	}

	err = tx.Commit()

	return err
}

func (m *TokenModel) DeleteAllForUser(userID int64) (int64, error) {
	query := `
		DELETE FROM tokens
		WHERE user_id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, userID)
	r, _ := result.RowsAffected()
	return r, err
}

func (m *TokenModel) DeleteExpiredTokens() (int64, error) {
	query := `
		DELETE FROM tokens
		WHERE expiry < strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query)
	r, _ := result.RowsAffected()
	return r, err
}
