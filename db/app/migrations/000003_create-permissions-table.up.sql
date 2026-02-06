CREATE TABLE IF NOT EXISTS permissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT UNIQUE NOT NULL
);

INSERT INTO permissions (code) VALUES ('admin:access'), ('admin:create'), ('user:access') ON CONFLICT (code) DO NOTHING;