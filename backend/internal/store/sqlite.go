package store

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/simonjohansson/kanban/backend/internal/model"
)

type SQLiteProjection struct {
	db *sql.DB
}

func NewSQLiteProjection(path string) (*SQLiteProjection, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	projection := &SQLiteProjection{db: db}
	if err := projection.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return projection, nil
}

func (p *SQLiteProjection) Close() error {
	return p.db.Close()
}

func (p *SQLiteProjection) init() error {
	_, err := p.db.Exec(`
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
  status TEXT NOT NULL,
  column_name TEXT NOT NULL,
  deleted INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  comments_count INTEGER NOT NULL,
  history_count INTEGER NOT NULL,
  UNIQUE(project_slug, number)
);
`)
	return err
}

func (p *SQLiteProjection) UpsertProject(project model.Project) error {
	_, err := p.db.Exec(`
INSERT INTO projects (slug, name, local_path, remote_url, next_card_seq, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
  name = excluded.name,
  local_path = excluded.local_path,
  remote_url = excluded.remote_url,
  next_card_seq = excluded.next_card_seq,
  created_at = excluded.created_at,
  updated_at = excluded.updated_at;
`,
		project.Slug,
		project.Name,
		project.LocalPath,
		project.RemoteURL,
		project.NextCardSeq,
		project.CreatedAt.UTC().Format(time.RFC3339),
		project.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (p *SQLiteProjection) UpsertCard(card model.Card) error {
	_, err := p.db.Exec(`
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
`,
		card.ID,
		card.ProjectSlug,
		card.Number,
		card.Title,
		card.Status,
		card.Column,
		boolToInt(card.Deleted),
		card.CreatedAt.UTC().Format(time.RFC3339),
		card.UpdatedAt.UTC().Format(time.RFC3339),
		len(card.Comments),
		len(card.History),
	)
	return err
}

func (p *SQLiteProjection) HardDeleteCard(projectSlug string, number int) error {
	_, err := p.db.Exec(`DELETE FROM cards WHERE project_slug = ? AND number = ?`, projectSlug, number)
	return err
}

func (p *SQLiteProjection) ListCards(projectSlug string, includeDeleted bool) ([]model.CardSummary, error) {
	query := `
SELECT id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
FROM cards
WHERE project_slug = ?`
	if !includeDeleted {
		query += ` AND deleted = 0`
	}
	query += ` ORDER BY number ASC`
	rows, err := p.db.Query(query, projectSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]model.CardSummary, 0)
	for rows.Next() {
		var (
			c       model.CardSummary
			deleted int
			created string
			updated string
		)
		if err := rows.Scan(&c.ID, &c.ProjectSlug, &c.Number, &c.Title, &c.Status, &c.Column, &deleted, &created, &updated, &c.CommentsCount, &c.HistoryCount); err != nil {
			return nil, err
		}
		c.Deleted = deleted == 1
		if c.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
			return nil, err
		}
		if c.UpdatedAt, err = time.Parse(time.RFC3339, updated); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

func (p *SQLiteProjection) RebuildFromMarkdown(projects []model.Project, cards []model.Card) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM cards`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM projects`); err != nil {
		return err
	}

	sort.Slice(projects, func(i, j int) bool { return projects[i].Slug < projects[j].Slug })
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].ProjectSlug == cards[j].ProjectSlug {
			return cards[i].Number < cards[j].Number
		}
		return cards[i].ProjectSlug < cards[j].ProjectSlug
	})

	for _, project := range projects {
		if _, err = tx.Exec(`
INSERT INTO projects (slug, name, local_path, remote_url, next_card_seq, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`,
			project.Slug,
			project.Name,
			project.LocalPath,
			project.RemoteURL,
			project.NextCardSeq,
			project.CreatedAt.UTC().Format(time.RFC3339),
			project.UpdatedAt.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("insert project %s: %w", project.Slug, err)
		}
	}

	for _, card := range cards {
		if _, err = tx.Exec(`
INSERT INTO cards (
  id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
			card.ID,
			card.ProjectSlug,
			card.Number,
			card.Title,
			card.Status,
			card.Column,
			boolToInt(card.Deleted),
			card.CreatedAt.UTC().Format(time.RFC3339),
			card.UpdatedAt.UTC().Format(time.RFC3339),
			len(card.Comments),
			len(card.History),
		); err != nil {
			return fmt.Errorf("insert card %s: %w", card.ID, err)
		}
	}

	return tx.Commit()
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
