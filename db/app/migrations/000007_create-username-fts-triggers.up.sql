CREATE TRIGGER users_ai AFTER INSERT ON users BEGIN
  INSERT INTO user_usernames_fts(rowid, username)
  VALUES (new.id, new.username);
END;

CREATE TRIGGER users_ad AFTER DELETE ON users BEGIN
  INSERT INTO user_usernames_fts(user_usernames_fts, rowid, username)
  VALUES('delete', old.id, old.username);
END;

CREATE TRIGGER users_au AFTER UPDATE ON users BEGIN
  INSERT INTO user_usernames_fts(user_usernames_fts, rowid, username)
  VALUES('delete', old.id, old.username);
  INSERT INTO user_usernames_fts(rowid, username)
  VALUES (new.id, new.username);
END;