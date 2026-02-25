CREATE VIRTUAL TABLE IF NOT EXISTS user_usernames_fts
  USING fts5(username, content='users', content_rowid='id', tokenize='unicode61 remove_diacritics 2');