package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/stretchr/testify/require"
)

func TestSQLiteProjectionLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "projection.db")
	p, err := NewSQLiteProjection(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	project := model.Project{
		Name:        "Alpha",
		Slug:        "alpha",
		LocalPath:   "/tmp/alpha",
		RemoteURL:   "git@example.com/alpha",
		CreatedAt:   now,
		UpdatedAt:   now,
		NextCardSeq: 2,
	}
	require.NoError(t, p.UpsertProject(project))

	card1 := model.Card{
		ID:          "alpha/card-1",
		ProjectSlug: "alpha",
		Number:      1,
		Title:       "Task A",
		Status:      "Todo",
		Deleted:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
		Comments:    []model.TextEvent{{Timestamp: now, Body: "comment"}},
		History:     []model.HistoryEvent{{Timestamp: now, Type: "card.created", Details: "status=Todo"}},
	}
	require.NoError(t, p.UpsertCard(card1))

	card2 := card1
	card2.ID = "alpha/card-2"
	card2.Number = 2
	card2.Title = "Task B"
	card2.Deleted = true
	require.NoError(t, p.UpsertCard(card2))

	active, err := p.ListCards("alpha", false)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, "alpha/card-1", active[0].ID)

	allCards, err := p.ListCards("alpha", true)
	require.NoError(t, err)
	require.Len(t, allCards, 2)

	require.NoError(t, p.HardDeleteCard("alpha", 2))
	allCards, err = p.ListCards("alpha", true)
	require.NoError(t, err)
	require.Len(t, allCards, 1)

	require.NoError(t, p.DeleteProject("alpha"))
	allCards, err = p.ListCards("alpha", true)
	require.NoError(t, err)
	require.Len(t, allCards, 0)
}

func TestSQLiteProjectionRebuildFromMarkdown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "projection.db")
	p, err := NewSQLiteProjection(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	projects := []model.Project{
		{Slug: "beta", Name: "Beta", CreatedAt: now, UpdatedAt: now, NextCardSeq: 1},
		{Slug: "alpha", Name: "Alpha", CreatedAt: now, UpdatedAt: now, NextCardSeq: 3},
	}
	cards := []model.Card{
		{ID: "beta/card-1", ProjectSlug: "beta", Number: 1, Title: "B", Status: "Doing", CreatedAt: now, UpdatedAt: now},
		{ID: "alpha/card-2", ProjectSlug: "alpha", Number: 2, Title: "A2", Status: "Review", CreatedAt: now, UpdatedAt: now},
		{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1, Title: "A1", Status: "Todo", CreatedAt: now, UpdatedAt: now},
	}

	require.NoError(t, p.RebuildFromMarkdown(projects, cards))

	alphaCards, err := p.ListCards("alpha", true)
	require.NoError(t, err)
	require.Len(t, alphaCards, 2)
	require.Equal(t, 1, alphaCards[0].Number)
	require.Equal(t, 2, alphaCards[1].Number)

	betaCards, err := p.ListCards("beta", true)
	require.NoError(t, err)
	require.Len(t, betaCards, 1)
	require.Equal(t, "beta/card-1", betaCards[0].ID)
}

func TestSQLiteHelperFunctions(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	summary, err := cardSummaryFromRaw("alpha/card-1", "alpha", 1, "Task", "Todo", 1, now.Format(time.RFC3339), now.Format(time.RFC3339), 2, 3)
	require.NoError(t, err)
	require.True(t, summary.Deleted)
	require.Equal(t, 2, summary.CommentsCount)
	require.Equal(t, 3, summary.HistoryCount)

	_, err = cardSummaryFromRaw("id", "alpha", 1, "Task", "Todo", 0, "bad", now.Format(time.RFC3339), 0, 0)
	require.Error(t, err)
	_, err = cardSummaryFromRaw("id", "alpha", 1, "Task", "Todo", 0, now.Format(time.RFC3339), "bad", 0, 0)
	require.Error(t, err)

	require.EqualValues(t, 1, boolToInt(true))
	require.EqualValues(t, 0, boolToInt(false))

	null := nullableString("")
	require.False(t, null.Valid)
	null = nullableString("value")
	require.True(t, null.Valid)
	require.Equal(t, "value", null.String)
}
