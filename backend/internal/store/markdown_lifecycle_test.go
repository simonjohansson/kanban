package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkdownStoreProjectAndCardLifecycle(t *testing.T) {
	s, err := NewMarkdownStore(t.TempDir())
	require.NoError(t, err)

	project, err := s.CreateProject("Alpha Project", "/tmp/alpha", "git@example.com/alpha")
	require.NoError(t, err)
	require.Equal(t, "alpha-project", project.Slug)
	require.Equal(t, 1, project.NextCardSeq)

	_, err = s.CreateProject("Alpha Project", "", "")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrExist))

	projects, err := s.ListProjects()
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "alpha-project", projects[0].Slug)

	loadedProject, err := s.GetProject("alpha-project")
	require.NoError(t, err)
	require.Equal(t, "Alpha Project", loadedProject.Name)

	card, err := s.CreateCard("alpha-project", "Task A", "first description", "Todo")
	require.NoError(t, err)
	require.Equal(t, "alpha-project/card-1", card.ID)
	require.Equal(t, "Todo", card.Status)
	require.Len(t, card.Description, 1)
	require.Len(t, card.History, 1)

	card, err = s.AppendDescription("alpha-project", 1, "more details")
	require.NoError(t, err)
	require.Len(t, card.Description, 2)
	require.Contains(t, card.History[len(card.History)-1].Details, "description")

	card, err = s.AddComment("alpha-project", 1, "looks good")
	require.NoError(t, err)
	require.Len(t, card.Comments, 1)
	require.Contains(t, card.History[len(card.History)-1].Type, "commented")

	card, err = s.MoveCard("alpha-project", 1, "Doing")
	require.NoError(t, err)
	require.Equal(t, "Doing", card.Status)

	softDeleted, err := s.DeleteCard("alpha-project", 1, false)
	require.NoError(t, err)
	require.True(t, softDeleted.Deleted)

	card2, err := s.CreateCard("alpha-project", "Task B", "", "Todo")
	require.NoError(t, err)
	require.Equal(t, 2, card2.Number)

	hardDeleted, err := s.DeleteCard("alpha-project", 2, true)
	require.NoError(t, err)
	require.Equal(t, 2, hardDeleted.Number)
	_, err = s.GetCard("alpha-project", 2)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))

	snapshotProjects, snapshotCards, err := s.Snapshot()
	require.NoError(t, err)
	require.Len(t, snapshotProjects, 1)
	require.Len(t, snapshotCards, 1)
	require.Equal(t, 1, snapshotCards[0].Number)
	require.True(t, snapshotCards[0].Deleted)

	require.NoError(t, s.DeleteProject("alpha-project"))
	_, err = s.GetProject("alpha-project")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestMarkdownStoreValidationAndParsingHelpers(t *testing.T) {
	s, err := NewMarkdownStore(t.TempDir())
	require.NoError(t, err)

	_, err = s.CreateProject("   ", "", "")
	require.ErrorContains(t, err, "name is required")

	_, err = s.CreateCard("missing", "Task", "", "Todo")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))

	project, err := s.CreateProject("Valid", "", "")
	require.NoError(t, err)
	require.Equal(t, "valid", project.Slug)

	_, err = s.CreateCard("valid", "", "", "Todo")
	require.ErrorContains(t, err, "title is required")

	_, err = s.CreateCard("valid", "Task", "", "Blocked")
	require.ErrorContains(t, err, "invalid status")

	card, err := s.CreateCard("valid", "Task", "", "Todo")
	require.NoError(t, err)
	require.Equal(t, 1, card.Number)

	_, err = s.AppendDescription("valid", 1, "   ")
	require.ErrorContains(t, err, "description body is required")

	_, err = s.AddComment("valid", 1, " ")
	require.ErrorContains(t, err, "comment body is required")

	_, err = s.MoveCard("valid", 1, "")
	require.ErrorContains(t, err, "status is required")

	_, err = s.GetCard("valid", 99)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))

	_, err = s.DeleteCard("valid", 99, false)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))

	yml, body, err := serializeCard(card)
	require.NoError(t, err)
	require.NotEmpty(t, yml)
	require.Contains(t, body, "# Description")

	roundTrip, err := parseCard(append(append([]byte("---\n"), yml...), []byte("---\n"+body)...))
	require.NoError(t, err)
	require.Equal(t, card.ID, roundTrip.ID)

	_, _, err = splitFrontmatter([]byte("not-frontmatter"))
	require.ErrorContains(t, err, "missing frontmatter")
	_, _, err = splitFrontmatter([]byte("---\na: b\n"))
	require.ErrorContains(t, err, "invalid frontmatter")

	require.NoError(t, validateStatus("Todo"))
	require.ErrorContains(t, validateStatus(""), "status is required")
	require.ErrorContains(t, validateStatus("invalid"), "invalid status")

	require.Equal(t, "hello-world", Slugify(" Hello, World "))
	require.Equal(t, "project", Slugify("***"))

	num, ok := cardNumberFromFilename("card-42.md")
	require.True(t, ok)
	require.Equal(t, 42, num)
	_, ok = cardNumberFromFilename("card-nope.md")
	require.False(t, ok)
	_, ok = cardNumberFromFilename("notes.md")
	require.False(t, ok)
}

func TestListProjectCardsSkipsInvalidCardFilenames(t *testing.T) {
	root := t.TempDir()
	s, err := NewMarkdownStore(root)
	require.NoError(t, err)

	_, err = s.CreateProject("Alpha", "", "")
	require.NoError(t, err)
	_, err = s.CreateCard("alpha", "Task", "", "Todo")
	require.NoError(t, err)

	projectDir := filepath.Join(root, "projects", "alpha")
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "card-not-a-number.md"), []byte("junk"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "readme.txt"), []byte("junk"), 0o644))

	cards, err := s.listProjectCards("alpha")
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Equal(t, 1, cards[0].Number)
}
