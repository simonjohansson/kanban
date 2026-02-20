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
	createCardFn        func(string, string, string, string, string) (model.Card, error)
	getProjectFn        func(string) (model.Project, error)
	getCardFn           func(string, int) (model.Card, error)
	moveCardFn          func(string, int, string) (model.Card, error)
	setCardBranchFn     func(string, int, string) (model.Card, error)
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

func (m *markdownStoreStub) CreateCard(projectSlug, title, description, branch, status string) (model.Card, error) {
	return m.createCardFn(projectSlug, title, description, branch, status)
}

func (m *markdownStoreStub) GetCard(projectSlug string, number int) (model.Card, error) {
	return m.getCardFn(projectSlug, number)
}

func (m *markdownStoreStub) MoveCard(projectSlug string, number int, status string) (model.Card, error) {
	return m.moveCardFn(projectSlug, number, status)
}

func (m *markdownStoreStub) SetCardBranch(projectSlug string, number int, branch string) (model.Card, error) {
	return m.setCardBranchFn(projectSlug, number, branch)
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
		createCardFn: func(projectSlug, _, _, _, _ string) (model.Card, error) {
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
	card, err := svc.CreateCard("alpha", "title", "", "", "Todo")
	require.NoError(t, err)
	require.Equal(t, "alpha/card-1", card.ID)
	require.True(t, projectUpserted)
	require.True(t, cardUpserted)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "card.created", publisher.events[0].Type)
}

func TestListProjectsMapsStoreErrorToInternal(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		listProjectsFn: func() ([]model.Project, error) {
			return nil, errors.New("boom")
		},
	}, &projectionStub{}, &publisherStub{})

	_, err := svc.ListProjects()
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestListCardsMapsProjectionErrorToInternal(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{}, &projectionStub{
		listCardsFn: func(_ string, _ bool) ([]model.CardSummary, error) {
			return nil, errors.New("boom")
		},
	}, &publisherStub{})

	_, err := svc.ListCards("alpha", false)
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestGetCardMapsNotFound(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		getCardFn: func(_ string, _ int) (model.Card, error) {
			return model.Card{}, os.ErrNotExist
		},
	}, &projectionStub{}, &publisherStub{})

	_, err := svc.GetCard("alpha", 1)
	require.Error(t, err)
	require.Equal(t, CodeNotFound, CodeOf(err))
}

func TestMoveCardSuccessPublishesEvent(t *testing.T) {
	t.Parallel()

	card := model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1, Status: "Doing"}
	publisher := &publisherStub{}
	svc := newNoopService(&markdownStoreStub{
		moveCardFn: func(_ string, _ int, _ string) (model.Card, error) {
			return card, nil
		},
	}, &projectionStub{
		upsertCardFn: func(input model.Card) error {
			require.Equal(t, card.ID, input.ID)
			return nil
		},
	}, publisher)

	got, err := svc.MoveCard("alpha", 1, "Doing")
	require.NoError(t, err)
	require.Equal(t, card.ID, got.ID)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "card.moved", publisher.events[0].Type)
}

func TestMoveCardProjectionFailureReturnsInternal(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		moveCardFn: func(_ string, _ int, _ string) (model.Card, error) {
			return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1, Status: "Doing"}, nil
		},
	}, &projectionStub{
		upsertCardFn: func(_ model.Card) error {
			return errors.New("projection down")
		},
	}, &publisherStub{})

	_, err := svc.MoveCard("alpha", 1, "Doing")
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestCommentCardSuccess(t *testing.T) {
	t.Parallel()

	card := model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}
	publisher := &publisherStub{}
	svc := newNoopService(&markdownStoreStub{
		addCommentFn: func(_ string, _ int, _ string) (model.Card, error) {
			return card, nil
		},
	}, &projectionStub{
		upsertCardFn: func(_ model.Card) error { return nil },
	}, publisher)

	_, err := svc.CommentCard("alpha", 1, "hello")
	require.NoError(t, err)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "card.commented", publisher.events[0].Type)
}

func TestAppendDescriptionSuccess(t *testing.T) {
	t.Parallel()

	card := model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}
	publisher := &publisherStub{}
	svc := newNoopService(&markdownStoreStub{
		appendDescriptionFn: func(_ string, _ int, _ string) (model.Card, error) {
			return card, nil
		},
	}, &projectionStub{
		upsertCardFn: func(_ model.Card) error { return nil },
	}, publisher)

	_, err := svc.AppendDescription("alpha", 1, "desc")
	require.NoError(t, err)
	require.Len(t, publisher.events, 1)
	require.Equal(t, "card.updated", publisher.events[0].Type)
}

func TestDeleteCardSoftAndHardPaths(t *testing.T) {
	t.Parallel()

	card := model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}
	publisher := &publisherStub{}
	svc := newNoopService(&markdownStoreStub{
		deleteCardFn: func(_ string, _ int, _ bool) (model.Card, error) {
			return card, nil
		},
	}, &projectionStub{
		upsertCardFn: func(_ model.Card) error { return nil },
		hardDeleteCardFn: func(_ string, _ int) error {
			return nil
		},
	}, publisher)

	_, err := svc.DeleteCard("alpha", 1, false)
	require.NoError(t, err)
	_, err = svc.DeleteCard("alpha", 1, true)
	require.NoError(t, err)
	require.Len(t, publisher.events, 2)
	require.Equal(t, "card.deleted_soft", publisher.events[0].Type)
	require.Equal(t, "card.deleted_hard", publisher.events[1].Type)
}

func TestDeleteCardProjectionFailureReturnsInternal(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		deleteCardFn: func(_ string, _ int, _ bool) (model.Card, error) {
			return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}, nil
		},
	}, &projectionStub{
		upsertCardFn: func(_ model.Card) error { return errors.New("projection down") },
		hardDeleteCardFn: func(_ string, _ int) error {
			return errors.New("projection down")
		},
	}, &publisherStub{})

	_, err := svc.DeleteCard("alpha", 1, false)
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))

	_, err = svc.DeleteCard("alpha", 1, true)
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestRebuildProjectionPaths(t *testing.T) {
	t.Parallel()

	projects := []model.Project{{Slug: "alpha"}}
	cards := []model.Card{{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}}

	svc := newNoopService(&markdownStoreStub{
		snapshotFn: func() ([]model.Project, []model.Card, error) {
			return projects, cards, nil
		},
	}, &projectionStub{
		rebuildFromMdFn: func(inProjects []model.Project, inCards []model.Card) error {
			require.Len(t, inProjects, 1)
			require.Len(t, inCards, 1)
			return nil
		},
	}, &publisherStub{})

	result, err := svc.RebuildProjection()
	require.NoError(t, err)
	require.Equal(t, 1, result.ProjectsRebuilt)
	require.Equal(t, 1, result.CardsRebuilt)
}

func TestRebuildProjectionErrors(t *testing.T) {
	t.Parallel()

	snapshotFail := newNoopService(&markdownStoreStub{
		snapshotFn: func() ([]model.Project, []model.Card, error) {
			return nil, nil, errors.New("snapshot failed")
		},
	}, &projectionStub{}, &publisherStub{})
	_, err := snapshotFail.RebuildProjection()
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))

	rebuildFail := newNoopService(&markdownStoreStub{
		snapshotFn: func() ([]model.Project, []model.Card, error) {
			return []model.Project{}, []model.Card{}, nil
		},
	}, &projectionStub{
		rebuildFromMdFn: func(_ []model.Project, _ []model.Card) error {
			return errors.New("rebuild failed")
		},
	}, &publisherStub{})
	_, err = rebuildFail.RebuildProjection()
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestErrorHelpers(t *testing.T) {
	t.Parallel()

	base := errors.New("inner")
	err := newError(CodeValidation, "invalid", base)
	require.Equal(t, "invalid", err.Error())
	require.ErrorIs(t, err, base)
	require.Equal(t, base, err.Unwrap())
	require.Equal(t, CodeValidation, CodeOf(err))
	require.Equal(t, "invalid", MessageOf(err))
	require.Equal(t, CodeInternal, CodeOf(errors.New("plain")))
	require.Equal(t, "plain", MessageOf(errors.New("plain")))
	require.Equal(t, "", MessageOf(nil))

	empty := &Error{Code: CodeInternal}
	require.Equal(t, "internal", empty.Error())
	require.Nil(t, (*Error)(nil).Unwrap())
	require.Equal(t, "", (*Error)(nil).Error())
}

func TestNewDefaultsLoggerWhenNil(t *testing.T) {
	t.Parallel()

	svc := New(&markdownStoreStub{}, &projectionStub{}, &publisherStub{}, nil)
	require.NotNil(t, svc.logger)
}

func TestCreateProjectProjectionFailureReturnsInternal(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		createProjectFn: func(_, _, _ string) (model.Project, error) {
			return model.Project{Slug: "alpha"}, nil
		},
	}, &projectionStub{
		upsertProjectFn: func(_ model.Project) error { return errors.New("projection down") },
	}, &publisherStub{})

	_, err := svc.CreateProject("alpha", "", "")
	require.Error(t, err)
	require.Equal(t, CodeInternal, CodeOf(err))
}

func TestCreateCardAdditionalErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("project not found maps validation", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			createCardFn: func(_, _, _, _, _ string) (model.Card, error) { return model.Card{}, os.ErrNotExist },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.CreateCard("alpha", "t", "", "", "Todo")
		require.Error(t, err)
		require.Equal(t, CodeValidation, CodeOf(err))
	})

	t.Run("get project failure maps internal", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			createCardFn: func(_, _, _, _, _ string) (model.Card, error) {
				return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}, nil
			},
			getProjectFn: func(_ string) (model.Project, error) { return model.Project{}, errors.New("boom") },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.CreateCard("alpha", "t", "", "", "Todo")
		require.Error(t, err)
		require.Equal(t, CodeInternal, CodeOf(err))
	})

	t.Run("project projection failure maps internal", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			createCardFn: func(_, _, _, _, _ string) (model.Card, error) {
				return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}, nil
			},
			getProjectFn: func(_ string) (model.Project, error) { return model.Project{Slug: "alpha"}, nil },
		}, &projectionStub{
			upsertProjectFn: func(_ model.Project) error { return errors.New("boom") },
		}, &publisherStub{})
		_, err := svc.CreateCard("alpha", "t", "", "", "Todo")
		require.Error(t, err)
		require.Equal(t, CodeInternal, CodeOf(err))
	})

	t.Run("card projection failure maps internal", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			createCardFn: func(_, _, _, _, _ string) (model.Card, error) {
				return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}, nil
			},
			getProjectFn: func(_ string) (model.Project, error) { return model.Project{Slug: "alpha"}, nil },
		}, &projectionStub{
			upsertProjectFn: func(_ model.Project) error { return nil },
			upsertCardFn:    func(_ model.Card) error { return errors.New("boom") },
		}, &publisherStub{})
		_, err := svc.CreateCard("alpha", "t", "", "", "Todo")
		require.Error(t, err)
		require.Equal(t, CodeInternal, CodeOf(err))
	})
}

func TestListAndGetCardPaths(t *testing.T) {
	t.Parallel()

	t.Run("list cards success", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{}, &projectionStub{
			listCardsFn: func(_ string, _ bool) ([]model.CardSummary, error) {
				return []model.CardSummary{{ID: "alpha/card-1"}}, nil
			},
		}, &publisherStub{})
		cards, err := svc.ListCards("alpha", false)
		require.NoError(t, err)
		require.Len(t, cards, 1)
	})

	t.Run("get card internal error", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			getCardFn: func(_ string, _ int) (model.Card, error) { return model.Card{}, errors.New("boom") },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.GetCard("alpha", 1)
		require.Error(t, err)
		require.Equal(t, CodeInternal, CodeOf(err))
	})
}

func TestMoveCommentAppendValidationAndNotFound(t *testing.T) {
	t.Parallel()

	t.Run("move not found", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			moveCardFn: func(_ string, _ int, _ string) (model.Card, error) { return model.Card{}, os.ErrNotExist },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.MoveCard("alpha", 1, "Doing")
		require.Error(t, err)
		require.Equal(t, CodeNotFound, CodeOf(err))
	})

	t.Run("move validation", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			moveCardFn: func(_ string, _ int, _ string) (model.Card, error) { return model.Card{}, errors.New("bad status") },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.MoveCard("alpha", 1, "Nope")
		require.Error(t, err)
		require.Equal(t, CodeValidation, CodeOf(err))
	})

	t.Run("comment not found", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			addCommentFn: func(_ string, _ int, _ string) (model.Card, error) { return model.Card{}, os.ErrNotExist },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.CommentCard("alpha", 1, "x")
		require.Error(t, err)
		require.Equal(t, CodeNotFound, CodeOf(err))
	})

	t.Run("comment projection failure", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			addCommentFn: func(_ string, _ int, _ string) (model.Card, error) {
				return model.Card{ID: "alpha/card-1", ProjectSlug: "alpha", Number: 1}, nil
			},
		}, &projectionStub{
			upsertCardFn: func(_ model.Card) error { return errors.New("boom") },
		}, &publisherStub{})
		_, err := svc.CommentCard("alpha", 1, "x")
		require.Error(t, err)
		require.Equal(t, CodeInternal, CodeOf(err))
	})

	t.Run("append not found", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			appendDescriptionFn: func(_ string, _ int, _ string) (model.Card, error) { return model.Card{}, os.ErrNotExist },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.AppendDescription("alpha", 1, "x")
		require.Error(t, err)
		require.Equal(t, CodeNotFound, CodeOf(err))
	})

	t.Run("append validation", func(t *testing.T) {
		svc := newNoopService(&markdownStoreStub{
			appendDescriptionFn: func(_ string, _ int, _ string) (model.Card, error) { return model.Card{}, errors.New("bad body") },
		}, &projectionStub{}, &publisherStub{})
		_, err := svc.AppendDescription("alpha", 1, "x")
		require.Error(t, err)
		require.Equal(t, CodeValidation, CodeOf(err))
	})
}

func TestDeleteCardValidationAndNotFound(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{
		deleteCardFn: func(_ string, _ int, _ bool) (model.Card, error) { return model.Card{}, os.ErrNotExist },
	}, &projectionStub{}, &publisherStub{})
	_, err := svc.DeleteCard("alpha", 1, false)
	require.Error(t, err)
	require.Equal(t, CodeNotFound, CodeOf(err))

	svc = newNoopService(&markdownStoreStub{
		deleteCardFn: func(_ string, _ int, _ bool) (model.Card, error) { return model.Card{}, errors.New("bad delete") },
	}, &projectionStub{}, &publisherStub{})
	_, err = svc.DeleteCard("alpha", 1, false)
	require.Error(t, err)
	require.Equal(t, CodeValidation, CodeOf(err))
}

func TestPublishNoPublisherAndTrimmedProject(t *testing.T) {
	t.Parallel()

	svc := newNoopService(&markdownStoreStub{}, &projectionStub{}, nil)
	svc.publish(model.Event{Project: " alpha "})

	publisher := &publisherStub{}
	svc = newNoopService(&markdownStoreStub{}, &projectionStub{}, publisher)
	svc.publish(model.Event{Type: "x", Project: " alpha "})
	require.Len(t, publisher.events, 1)
	require.Equal(t, "alpha", publisher.events[0].Project)
}
