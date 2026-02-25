CREATE TABLE sessions (
  id TEXT PRIMARY KEY,          -- the opaque random token
  user_id INTEGER,
  ip_addr TEXT,
  created_at DATETIME NOT NULL,
  last_seen_at DATETIME NOT NULL,
  expiry DATETIME NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);