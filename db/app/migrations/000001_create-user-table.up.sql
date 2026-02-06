CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uuid TEXT NOT NULL UNIQUE,
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  username TEXT NOT NULL UNIQUE,
  email TEXT UNIQUE,
  google_id TEXT UNIQUE,
  password_hash BLOB NOT NULL,
  activated INTEGER NOT NULL DEFAULT 0 CHECK(activated IN (0,1)),
  last_authenticated_at DATETIME NOT NULL DEFAULT (datetime('now')),
  version INTEGER NOT NULL DEFAULT 1
);