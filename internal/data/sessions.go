package data

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

type Session struct {
	ID         string
	UserID     *int64
	IPAddr     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	Expiry     time.Time
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

type SessionModel struct {
	DB *sql.DB
}

func (m *SessionModel) New(ctx context.Context, userID *int64, ipAddr string, ttl time.Duration) (*Session, error) {
	token, err := generateSessionToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	session := &Session{
		ID:         token,
		UserID:     userID,
		IPAddr:     ipAddr,
		CreatedAt:  now,
		LastSeenAt: now,
		Expiry:     now.Add(ttl),
	}

	_, err = m.DB.ExecContext(ctx, `
			INSERT INTO sessions (id, user_id, ip_addr, created_at, last_seen_at, expiry)
			VALUES ($1, $2, $3, $4, $5, $6);`,
		session.ID, session.UserID, session.IPAddr, session.CreatedAt,
		session.LastSeenAt, session.Expiry,
	)
	return session, err
}

func (m *SessionModel) Get(ctx context.Context, id string) (*Session, error) {
	session := &Session{}
	err := m.DB.QueryRowContext(ctx, `
		SELECT id, user_id, ip_addr, created_at, last_seen_at, expiry
		FROM sessions WHERE id = $1;`, id,
	).Scan(&session.ID, &session.UserID, &session.IPAddr, &session.CreatedAt,
		&session.LastSeenAt, &session.Expiry)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, ProcessSQLError(err, "get session by id")
		}
	}

	return session, nil
}

func (m *SessionModel) AttachUser(ctx context.Context, sessionID string, userID int64) error {
	_, err := m.DB.ExecContext(ctx, `
			UPDATE sessions 
			SET user_id = $1
			WHERE id = $2;`,
		userID, sessionID,
	)
	return err
}

func (m *SessionModel) Delete(ctx context.Context, id string) error {
	_, err := m.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1;`, id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return ProcessSQLError(err, "delete session by ID")
		}
	}

	return nil
}

func (m *SessionModel) DeleteAllForUser(ctx context.Context, userID int64) error {
	_, err := m.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1;`, userID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return ProcessSQLError(err, "delete all sessions for user")
		}
	}

	return nil
}

func (m *SessionModel) UpdateLastSeen(ctx context.Context, id string) error {
	_, err := m.DB.ExecContext(ctx, `
			UPDATE sessions SET last_seen_at = $1 WHERE id = $2;`,
		time.Now().UTC(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return ProcessSQLError(err, "update last seen for session")
		}
	}

	return nil
}

func (m *SessionModel) DeleteExpiredSessions() (int64, error) {
	query := `
		DELETE FROM sessions
		WHERE expiry < strftime('%Y-%m-%dT%H:%M:%SZ', 'now');
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query)
	r, _ := result.RowsAffected()
	return r, err
}
