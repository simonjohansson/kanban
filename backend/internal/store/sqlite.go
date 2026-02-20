package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/simonjohansson/kanban/backend/internal/store/sqlcgen"
)

type SQLiteProjection struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewSQLiteProjection(path string) (*SQLiteProjection, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	projection := &SQLiteProjection{db: db, queries: sqlcgen.New(db)}
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
	ctx := context.Background()
	if err := p.queries.InitProjectsTable(ctx); err != nil {
		return err
	}
	return p.queries.InitCardsTable(ctx)
}

func (p *SQLiteProjection) UpsertProject(project model.Project) error {
	return p.queries.UpsertProject(context.Background(), sqlcgen.UpsertProjectParams{
		Slug:        project.Slug,
		Name:        project.Name,
		LocalPath:   nullableString(project.LocalPath),
		RemoteUrl:   nullableString(project.RemoteURL),
		NextCardSeq: int64(project.NextCardSeq),
		CreatedAt:   project.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   project.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

func (p *SQLiteProjection) UpsertCard(card model.Card) error {
	return p.queries.UpsertCard(context.Background(), sqlcgen.UpsertCardParams{
		ID:            card.ID,
		ProjectSlug:   card.ProjectSlug,
		Number:        int64(card.Number),
		Title:         card.Title,
		Status:        card.Status,
		ColumnName:    card.Column,
		Deleted:       boolToInt(card.Deleted),
		CreatedAt:     card.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     card.UpdatedAt.UTC().Format(time.RFC3339),
		CommentsCount: int64(len(card.Comments)),
		HistoryCount:  int64(len(card.History)),
	})
}

func (p *SQLiteProjection) HardDeleteCard(projectSlug string, number int) error {
	return p.queries.HardDeleteCard(context.Background(), sqlcgen.HardDeleteCardParams{
		ProjectSlug: projectSlug,
		Number:      int64(number),
	})
}

func (p *SQLiteProjection) ListCards(projectSlug string, includeDeleted bool) ([]model.CardSummary, error) {
	ctx := context.Background()
	if includeDeleted {
		rows, err := p.queries.ListCardsWithDeleted(ctx, projectSlug)
		if err != nil {
			return nil, err
		}
		return mapCardSummaryRowsFromAll(rows)
	}
	rows, err := p.queries.ListCardsActive(ctx, projectSlug)
	if err != nil {
		return nil, err
	}
	return mapCardSummaryRowsFromActive(rows)
}

func (p *SQLiteProjection) RebuildFromMarkdown(projects []model.Project, cards []model.Card) error {
	sort.Slice(projects, func(i, j int) bool { return projects[i].Slug < projects[j].Slug })
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].ProjectSlug == cards[j].ProjectSlug {
			return cards[i].Number < cards[j].Number
		}
		return cards[i].ProjectSlug < cards[j].ProjectSlug
	})

	tx, err := p.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	qtx := p.queries.WithTx(tx)
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = qtx.DeleteAllCards(context.Background()); err != nil {
		return err
	}
	if err = qtx.DeleteAllProjects(context.Background()); err != nil {
		return err
	}

	for _, project := range projects {
		if err = qtx.InsertProject(context.Background(), sqlcgen.InsertProjectParams{
			Slug:        project.Slug,
			Name:        project.Name,
			LocalPath:   nullableString(project.LocalPath),
			RemoteUrl:   nullableString(project.RemoteURL),
			NextCardSeq: int64(project.NextCardSeq),
			CreatedAt:   project.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:   project.UpdatedAt.UTC().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("insert project %s: %w", project.Slug, err)
		}
	}

	for _, card := range cards {
		if err = qtx.InsertCard(context.Background(), sqlcgen.InsertCardParams{
			ID:            card.ID,
			ProjectSlug:   card.ProjectSlug,
			Number:        int64(card.Number),
			Title:         card.Title,
			Status:        card.Status,
			ColumnName:    card.Column,
			Deleted:       boolToInt(card.Deleted),
			CreatedAt:     card.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:     card.UpdatedAt.UTC().Format(time.RFC3339),
			CommentsCount: int64(len(card.Comments)),
			HistoryCount:  int64(len(card.History)),
		}); err != nil {
			return fmt.Errorf("insert card %s: %w", card.ID, err)
		}
	}

	return tx.Commit()
}

func mapCardSummaryRowsFromActive(rows []sqlcgen.Card) ([]model.CardSummary, error) {
	cards := make([]model.CardSummary, 0, len(rows))
	for _, row := range rows {
		card, err := cardSummaryFromRaw(
			row.ID,
			row.ProjectSlug,
			row.Number,
			row.Title,
			row.Status,
			row.ColumnName,
			row.Deleted,
			row.CreatedAt,
			row.UpdatedAt,
			row.CommentsCount,
			row.HistoryCount,
		)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func mapCardSummaryRowsFromAll(rows []sqlcgen.Card) ([]model.CardSummary, error) {
	cards := make([]model.CardSummary, 0, len(rows))
	for _, row := range rows {
		card, err := cardSummaryFromRaw(
			row.ID,
			row.ProjectSlug,
			row.Number,
			row.Title,
			row.Status,
			row.ColumnName,
			row.Deleted,
			row.CreatedAt,
			row.UpdatedAt,
			row.CommentsCount,
			row.HistoryCount,
		)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func cardSummaryFromRaw(id, projectSlug string, number int64, title, status, column string, deleted int64, created, updated string, commentsCount, historyCount int64) (model.CardSummary, error) {
	createdAt, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return model.CardSummary{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339, updated)
	if err != nil {
		return model.CardSummary{}, err
	}
	return model.CardSummary{
		ID:            id,
		ProjectSlug:   projectSlug,
		Number:        int(number),
		Title:         title,
		Status:        status,
		Column:        column,
		Deleted:       deleted == 1,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		CommentsCount: int(commentsCount),
		HistoryCount:  int(historyCount),
	}, nil
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func nullableString(v string) sql.NullString {
	trimmed := v
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
