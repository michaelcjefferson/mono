CREATE TABLE IF NOT EXISTS tokens (
  hash BLOB PRIMARY KEY, 
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, 
  expiry DATETIME NOT NULL,
  scope TEXT NOT NULL CHECK (scope IN ('activation', 'authentication'))
);

-- Index expiry so that token deletion cycle can more efficiently find tokens to be deleted
CREATE INDEX idx_tokens_expiry ON tokens(expiry);