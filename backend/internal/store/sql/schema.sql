CREATE TABLE IF NOT EXISTS projects (
  slug TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  local_path TEXT,
  remote_url TEXT,
  next_card_seq INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  project_slug TEXT NOT NULL,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  branch TEXT,
  status TEXT NOT NULL,
  deleted INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  comments_count INTEGER NOT NULL,
  history_count INTEGER NOT NULL,
  UNIQUE(project_slug, number)
);
