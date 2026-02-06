package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"placeholder_project_tag/pkg/logging"
)

type LogModel struct {
	background func(fn func())
	DB         *sql.DB
}

// Insert an individual log into the logs table, as well as a reference to the log message for text search into the logs_fts table
func (m *LogModel) Insert(log *logging.Log) error {
	// Get user_id if it exists to add to logs_metadata table
	var userID int
	if i, ok := ToInt(log.Details["user_id"]); ok {
		userID = i
	}

	query := `
		INSERT INTO logs (level, timestamp, message, details, trace)
		VALUES ($1, $2, $3, $4, $5);

		INSERT INTO logs_fts (rowid, message)
		VALUES (last_insert_rowid(), $6);

		INSERT INTO logs_metadata (type, level, count)
		VALUES ("level", $7, 1)
		ON CONFLICT(level) DO UPDATE
		SET count=count+1;
	`

	jsonProps, err := json.Marshal(log.Details)
	if err != nil {
		fmt.Println("error marshalling json when attempting to write a log to database:", err)
		return err
	}

	args := []any{log.Level, log.Timestamp, log.Message, jsonProps, log.Trace, log.Message, log.Level}

	if userID > 0 {
		query += `
			INSERT INTO logs_metadata (type, user_id, count)
			VALUES ("user_id", $8, 1)
			ON CONFLICT(user_id) DO UPDATE
			SET count=count+1;
		`

		args = append(args, userID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = m.DB.ExecContext(ctx, query, args...)

	if err != nil {
		fmt.Printf("Error pushing log to SQLite database: %v", err)
	}

	return err
}

func (m *LogModel) GetForID(id int) (*logging.Log, error) {
	// Seeing as all logs that exist in the database will have a positive integer as their id, check that the request id is valid before querying database to prevent wasted queries
	// IMPORTANT: Though it may seem like a good idea to use an unsigned int here (seeing as id will never be negative), it is more important that the types we use in our code align with the types available in our database. SQLite doesn't have unsigned ints, so use a standard int which is a reflection of SQLite's integer type
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, level, timestamp, message, details, trace, user_id
		FROM logs
		WHERE id = ?
	`

	// Declare Log struct to hold data returned by query
	var log logging.Log
	// Declare detailsJSON string to hold the details value returned by the query, so that it can be unmarshalled before being attached to the Log struct
	var detailsJSON string
	// Declare string to hold time value from database, which will be converted into time.Time
	// var timeStr string

	// Create an empty context (Background()) with a 3 second timeout. The timeout countdown will begin as soon as it is created
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// IMPORTANT: defer the cancel() function returned by context.WithTimeout(), so that in case of a successful request, the context is cancelled and resources are freed up before the request returns
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.Level,
		&log.Timestamp,
		&log.Message,
		&detailsJSON,
		&log.Trace,
		&log.UserID,
	)

	// Unmarshal details into log struct
	if detailsJSON != "" {
		err = json.Unmarshal([]byte(detailsJSON), &log.Details)
	}

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &log, nil
}

// Get all logs that match the provided filters, and return them along with metadata i.e. count of each type of log
func (m *LogModel) GetAll(filters LogFilters) ([]*logging.Log, *LogsMetadata, error) {
	// It's not possible to interpolate ORDER BY column or direction into an SQL query using $ values, so use Sprintf to create the query.
	// Subquery SELECT COUNT(*) FROM logs_fts provides the total number of rows returned by the query, and appends it to each row in the location specified (in this case, it is the last column of each row, i.e. after trace)
	// The JOIN also uses the logs_fts table to perform a search for messages that contain the provided searchTerm

	var queryBuilder strings.Builder
	args := []any{}

	queryBuilder.WriteString(`
		SELECT logs.id, logs.level, logs.timestamp, logs.message, logs.details, logs.user_id,
			(SELECT COUNT(*) FROM logs JOIN logs_fts ON logs.id = logs_fts.rowid WHERE 1=1
	`)

	getAllLogsFilterQueryHelper(&queryBuilder, &args, filters)

	queryBuilder.WriteString(") AS total_count FROM logs JOIN logs_fts ON logs.id = logs_fts.rowid WHERE 1=1")

	getAllLogsFilterQueryHelper(&queryBuilder, &args, filters)

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s, id DESC LIMIT ? OFFSET ?", filters.sortColumn(), filters.sortDirection()))
	args = append(args, filters.limit(), filters.offset())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		log.Printf("error getting logs from db: %v\n", err)
		return nil, nil, err
	}

	// Make sure result from QueryContext is closed before returning from function
	defer rows.Close()

	totalRecords := 0
	logs := []*logging.Log{}

	for rows.Next() {
		var log logging.Log
		var detailsJSON string
		var userID *int
		// var timeStr string

		err := rows.Scan(
			&log.ID,
			&log.Level,
			&log.Timestamp,
			&log.Message,
			&detailsJSON,
			&userID,
			&totalRecords,
		)
		if err != nil {
			return nil, nil, err
		}

		// Unmarshal details into log struct
		if detailsJSON != "" {
			err = json.Unmarshal([]byte(detailsJSON), &log.Details)
		}
		if err != nil {
			return nil, nil, err
		}

		// Attach value of user_id if it isn't nil
		if userID != nil {
			log.UserID = *userID
		}

		logs = append(logs, &log)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	logsMetadata, err := GetLogsMetadata(m)
	if err != nil {
		return logs, nil, err
	}

	logsMetadata.FilterMetadata = metadata

	return logs, logsMetadata, nil
}

// Dynamically build filters for log query, based on filters provided
func getAllLogsFilterQueryHelper(q *strings.Builder, args *[]any, filters LogFilters) {
	if filters.Message != "" {
		q.WriteString(" AND logs_fts MATCH ?")
		*args = append(*args, filters.Message)
	}
	if len(filters.Level) > 0 {
		qp := fmt.Sprintf(" AND level IN (%s)", Placeholders(len(filters.Level)))
		q.WriteString(qp)
		// q.WriteString(" AND level = ?")
		for _, val := range filters.Level {
			*args = append(*args, val)
		}
	}
	if len(filters.UserID) > 0 {
		// q.WriteString(" AND user_id = ?")
		qp := fmt.Sprintf(" AND user_id IN (%s)", Placeholders(len(filters.UserID)))
		q.WriteString(qp)
		for _, val := range filters.UserID {
			*args = append(*args, val)
		}
	}
}

// Get information about how many logs there are total, and how many logs are connected to each level and user
func GetLogsMetadata(m *LogModel) (*LogsMetadata, error) {
	query := `
		SELECT type, level, user_id, count FROM logs_metadata;
	`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return &LogsMetadata{}, err
	}

	logsMetadata := NewLogsMetadata()

	for rows.Next() {
		var logType string
		var level *string
		var userID *int
		var count int

		err := rows.Scan(
			&logType,
			&level,
			&userID,
			&count,
		)

		if err != nil {
			return &LogsMetadata{}, err
		}

		switch {
		case logType == "level" && level != nil:
			logsMetadata.Levels[*level] = count
		case logType == "user_id" && userID != nil:
			logsMetadata.Users[*userID] = count
		default:
			return &LogsMetadata{}, errors.New(fmt.Sprintf("error adding logtype %v to database", logType))
		}
	}

	if err = rows.Err(); err != nil {
		return &LogsMetadata{}, err
	}

	return &logsMetadata, nil
}

type metadataUpdates struct {
	Levels  map[string]int
	UserIDs map[int]int
}

func (m *LogModel) DeleteForID(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID *int
	var level *string

	log := tx.QueryRowContext(ctx,
		`SELECT user_id, level FROM logs
		WHERE id = ?`, id)
	err = log.Scan(&userID, &level)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM logs
		WHERE id = ?`, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE logs_metadata
		SET count = count - 1
		WHERE (level = ? OR user_id = ?)
		AND count > 0;
	`, level, userID)
	if err != nil {
		return err
	}

	// Remove rows where count is 0, so that those filters no longer appear on the Logs page
	_, err = tx.Exec(`
		DELETE FROM logs_metadata
		WHERE count = 0;
	`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

type LogDeletionParams struct {
	StartTime *time.Time
	EndTime   *time.Time
}

// Pointer to time.Time to allow for easy nil value checking
// TODO: Allow deletion by level and userID too
func (m *LogModel) DeleteAllInTimeRange(params LogDeletionParams) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build array of optional additions to query, so that start time and end time are not required
	var where []string
	var args []any

	if params.StartTime != nil {
		where = append(where, "timestamp >= ?")
		args = append(args, *params.StartTime)
	}
	if params.EndTime != nil {
		where = append(where, "timestamp <= ?")
		args = append(args, *params.EndTime)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	query := fmt.Sprintf(`
	SELECT level, user_id, COUNT(*)
	FROM logs
	%s
	GROUP BY level, user_id`, whereClause)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	updates := metadataUpdates{
		Levels:  make(map[string]int),
		UserIDs: make(map[int]int),
	}

	for rows.Next() {
		var level *string
		var userID *int
		var count int
		if err := rows.Scan(&level, &userID, &count); err != nil {
			return err
		}
		if level != nil {
			updates.Levels[*level] += count
		}
		if userID != nil {
			updates.UserIDs[*userID] += count
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	delQuery := fmt.Sprintf(`DELETE FROM logs %s`, whereClause)
	_, err = tx.ExecContext(ctx, delQuery, args...)
	if err != nil {
		return err
	}

	for level, count := range updates.Levels {
		_, err := tx.ExecContext(ctx, `
			UPDATE logs_metadata
			SET count = count - ?
			WHERE level = ?
		;`, count, level)
		if err != nil {
			return err
		}
	}
	for userID, count := range updates.UserIDs {
		_, err := tx.ExecContext(ctx, `
			UPDATE logs_metadata
			SET count = count - ?
			WHERE user_id = ?
		;`, count, userID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		DELETE FROM logs_metadata
		WHERE count = 0;
	`)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
