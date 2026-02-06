CREATE TABLE IF NOT EXISTS users_permissions (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  -- A composite primary key, composed of the user_id combined with the permission_id, prevents duplicated combinations in this table
  PRIMARY KEY (user_id, permission_id)
);

CREATE INDEX idx_users_permissions_permission_id
ON users_permissions(permission_id);