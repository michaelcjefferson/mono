CREATE TABLE IF NOT EXISTS users_permissions (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission_code TEXT NOT NULL,
  granted_at DATETIME NOT NULL DEFAULT (datetime('now')),
  -- A composite primary key, composed of the user_id combined with the permission_code, prevents duplicated combinations in this table
  PRIMARY KEY (user_id, permission_code)
);

CREATE INDEX idx_users_permissions_permission_code
ON users_permissions(permission_code);