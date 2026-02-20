package service

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/stretchr/testify/require"
)

type markdownStoreStub struct {
	deleteProjectFn     func(string) error
	createProjectFn     func(string, string, string) (model.Project, error)
	listProjectsFn      func() ([]model.Project, error)
	createCardFn        func(string, string, string, string) (model.Card, error)
	getProjectFn        func(string) (model.Project, error)
	getCardFn           func(string, int) (model.Card, error)
	moveCardFn          func(string, int, string) (model.Card, error)
	addCommentFn        func(string, int, string) (model.Card, error)
	appendDescriptionFn func(string, int, string) (model.Card, error)
	deleteCardFn        func(string, int, bool) (model.Card, error)
	snapshotFn          func() ([]model.Project, []model.Card, error)
}

func (m *markdownStoreStub) CreateProject(name, localPath, remoteURL string) (model.Project, error) {
	return m.createProjectFn(name, localPath, remoteURL)
}

func (m *markdownStoreStub) ListProjects() ([]model.Project, error) {
	return m.listProjectsFn()
}

func (m *markdownStoreStub) GetProject(slug string) (model.Project, error) {
	return m.getProjectFn(slug)
}

func (m *markdownStoreStub) DeleteProject(slug string) error {
	return m.deleteProjectFn(slug)
}

func (m *markdownStoreStub) CreateCard(projectSlug, title, description, status string) (model.Card, error) {
	return m.createCardFn(projectSlug, title, description, status)
}

func (m *markdownStoreStub) GetCard(projectSlug string, number int) (model.Card, error) {
	return m.getCardFn(projectSlug, number)
}

func (m *markdownStoreStub) MoveCard(projectSlug string, number int, status string) (model.Card, error) {
	return m.moveCardFn(projectSlug, number, status)
}

func (m *markdownStoreStub) AddComment(projectSlug string, number int, body string) (model.Card, error) {
	return m.addCommentFn(projectSlug, number, body)
}

func (m *markdownStoreStub) AppendDescription(projectSlug string, number int, body string) (model.Card, error) {
	return m.appendDescriptionFn(projectSlug, number, body)
}

func (m *markdownStoreStub) DeleteCard(projectSlug string, number int, hard bool) (model.Card, error) {
	return m.deleteCardFn(projectSlug, number, hard)
}

func (m *markdownStoreStub) Snapshot() ([]model.Project, []model.Card, error) {
	return m.snapshotFn()
}

type projectionStub struct {
	upsertProjectFn  func(model.Project) error
	upsertCardFn     func(model.Card) error
	deleteProjectFn  func(string) error
	hardDeleteCardFn func(string, int) error
	listCardsFn      func(string, bool) ([]model.CardSummary, error)
	rebuildFromMdFn  func([]model.Project, []model.Card) error
}

func (p *projectionStub) UpsertProject(project model.Project) error {
	return p.upsertProjectFn(project)
}
func (p *projectionStub) UpsertCard(card model.Card) error { return p.upsertCardFn(card) }
func (p *projectionStub) DeleteProject(projectSlug string) error {
	return p.deleteProjectFn(projectSlug)
}
func (p *projectionStub) HardDeleteCard(projectSlug string, number int) error {
	return p.hardDeleteCardFn(projectSlug, number)
}
func (p *projectionStub) ListCards(projectSlug string, includeDeleted bool) ([]model.CardSummary, error) {
	return p.listCardsFn(projectSlug, includeDeleted)
}
func (p *projectionStub) RebuildFromMarkdown(projects []model.Project, cards []model.Card) error {
	return p.rebuildFromMdFn(projects, cards)
}

type publisherStub struct {
	events []model.Event
}

func (p *publisherStub) Publish(event model.Event) {
	p.events = append(p.events, event)
}

func newNoopService(markdown MarkdownStore, projection Projection, publisher Publisher) *Service {
	return New(markdown, projection, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestDeleteProjectPublishesEventAfterProjectionSync(t *testing.T) {
	t.Parallel()

	var (
		markdownDeleted   string
		projectionDeleted string
	)

	markdown := &markdownStoreStub{
		deleteProjectFn: func(slug string) error {
			markdownDeleted = slug
			return nil
		},
	}
	projection := &projectionStub{
		deleteProjectFn: func(slug string) error {
			projectionDeleted = slug
			return nil
		},
	}
	publisher := &publisherStub{}

	svc := newNoopService(markdown, projection, publisher)
	err := svc.DeleteProject("alpha")
	require.NoError(t, err)
	require.Equal(t, "alpha", markdownDeleted)
	require.Equal(t, "alpha", projectionDeleted)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "project.deleted", publisher.events[0].Type)
	require.Equal(t, "alpha", publisher.events[0].Project)
}

func TestDeleteProjectReturnsInternalWhenProjectionFails(t *testing.T) {
	t.Parallel()

	markdown := &markdownStoreStub{
		deleteProjectFn: func(_ string) error {
			return nil
		},
	}
	projection := &projectionStub{
		deleteProjectFn: func(_ string) error {
			return errors.New("projection down")
		},
	}
	publisher := &publisherStub{}

	svc := newNoopService(markdown, projection, publisher)
	err := svc.DeleteProject("alpha")
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
	require.Len(t, publisher.events, 0)
}

func TestCreateProjectConflictMapping(t *testing.T) {
	t.Parallel()

	markdown := &markdownStoreStub{
		createProjectFn: func(_, _, _ string) (model.Project, error) {
			return model.Project{}, os.ErrExist
		},
	}
	svc := newNoopService(markdown, &projectionStub{}, &publisherStub{})
	_, err := svc.CreateProject("alpha", "", "")
	require.Error(t, err)
	require.Equal(t, CodeConflict, CodeOf(err))
}

func TestCreateCardUpsertsProjectAndCard(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	createdCard := model.Card{
		ID:          "alpha/card-1",
		ProjectSlug: "alpha",
		Number:      1,
		Status:      "Todo",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	project := model.Project{
		Name:        "Alpha",
		Slug:        "alpha",
		NextCardSeq: 2,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	var (
		projectUpserted bool
		cardUpserted    bool
	)

	markdown := &markdownStoreStub{
		createCardFn: func(projectSlug, _, _, _ string) (model.Card, error) {
			require.Equal(t, "alpha", projectSlug)
			return createdCard, nil
		},
		getProjectFn: func(slug string) (model.Project, error) {
			require.Equal(t, "alpha", slug)
			return project, nil
		},
	}
	projection := &projectionStub{
		upsertProjectFn: func(project model.Project) error {
			require.Equal(t, "alpha", project.Slug)
			projectUpserted = true
			return nil
		},
		upsertCardFn: func(card model.Card) error {
			require.Equal(t, "alpha/card-1", card.ID)
			cardUpserted = true
			return nil
		},
	}
	publisher := &publisherStub{}

	svc := newNoopService(markdown, projection, publisher)
	card, err := svc.CreateCard("alpha", "title", "", "Todo")
	require.NoError(t, err)
	require.Equal(t, "alpha/card-1", card.ID)
	require.True(t, projectUpserted)
	require.True(t, cardUpserted)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "card.created", publisher.events[0].Type)
}
