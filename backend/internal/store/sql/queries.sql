-- name: InitProjectsTable :exec
CREATE TABLE IF NOT EXISTS projects (
  slug TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  local_path TEXT,
  remote_url TEXT,
  next_card_seq INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- name: InitCardsTable :exec
CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  project_slug TEXT NOT NULL,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  column_name TEXT NOT NULL,
  deleted INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  comments_count INTEGER NOT NULL,
  history_count INTEGER NOT NULL,
  UNIQUE(project_slug, number)
);

-- name: UpsertProject :exec
INSERT INTO projects (slug, name, local_path, remote_url, next_card_seq, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
  name = excluded.name,
  local_path = excluded.local_path,
  remote_url = excluded.remote_url,
  next_card_seq = excluded.next_card_seq,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at;

-- name: UpsertCard :exec
INSERT INTO cards (
  id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  project_slug = excluded.project_slug,
  number = excluded.number,
  title = excluded.title,
  status = excluded.status,
  column_name = excluded.column_name,
  deleted = excluded.deleted,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at,
  comments_count = excluded.comments_count,
  history_count = excluded.history_count;

-- name: HardDeleteCard :exec
DELETE FROM cards WHERE project_slug = ? AND number = ?;

-- name: DeleteCardsByProject :exec
DELETE FROM cards WHERE project_slug = ?;

-- name: DeleteProjectBySlug :exec
DELETE FROM projects WHERE slug = ?;

-- name: ListCardsActive :many
SELECT id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
FROM cards
WHERE project_slug = ? AND deleted = 0
ORDER BY number ASC;

-- name: ListCardsWithDeleted :many
SELECT id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
FROM cards
WHERE project_slug = ?
ORDER BY number ASC;

-- name: DeleteAllCards :exec
DELETE FROM cards;

-- name: DeleteAllProjects :exec
DELETE FROM projects;

-- name: InsertProject :exec
INSERT INTO projects (slug, name, local_path, remote_url, next_card_seq, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: InsertCard :exec
INSERT INTO cards (
  id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
