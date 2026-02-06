CREATE TABLE IF NOT EXISTS logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  level TEXT NOT NULL,
  timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
  message TEXT NOT NULL,
  details TEXT,
  trace TEXT
);

-- Create an indexed column based on any logs that come with an attached user id, to make it easier to query for logs regarding a specific user. VIRTUAL allows the column to store NULL values without errors, and NULL values are ignored in indexes
-- Check the datatype of $.user_id before extracting, as otherwise in some cases the value of this column can be set to a single " and cause errors
-- NOTE: Even with the below query, details with a user_id value of "" still cause malformed JSON errors (specifically the logs created by logsPage query params)
ALTER TABLE logs ADD COLUMN user_id INTEGER
GENERATED ALWAYS AS (
  CASE 
    WHEN json_valid(details) 
      AND json_type(json_extract(details, '$.user_id')) = 'integer' 
      AND json_extract(details, '$.user_id') != '' 
    THEN json_extract(details, '$.user_id') 
    ELSE NULL 
  END
) VIRTUAL;

CREATE INDEX IF NOT EXISTS idx_logs_user_id ON logs(user_id);

-- FTS5 tables are optimised for text search, and allow in this case for more efficiently searching for logs containing key words
-- This particular FTS5 table is only set up to allow for searching for text in log messages only, as defined by the message param
CREATE VIRTUAL TABLE IF NOT EXISTS logs_fts
  USING fts5(message, content='logs', content_rowid='id');