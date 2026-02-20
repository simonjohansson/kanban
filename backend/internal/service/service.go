package service

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/model"
)

type MarkdownStore interface {
	CreateProject(name, localPath, remoteURL string) (model.Project, error)
	ListProjects() ([]model.Project, error)
	GetProject(slug string) (model.Project, error)
	DeleteProject(slug string) error
	CreateCard(projectSlug, title, description, status, column string) (model.Card, error)
	GetCard(projectSlug string, number int) (model.Card, error)
	MoveCard(projectSlug string, number int, status, column string) (model.Card, error)
	AddComment(projectSlug string, number int, body string) (model.Card, error)
	AppendDescription(projectSlug string, number int, body string) (model.Card, error)
	DeleteCard(projectSlug string, number int, hard bool) (model.Card, error)
	Snapshot() ([]model.Project, []model.Card, error)
}

type Projection interface {
	UpsertProject(project model.Project) error
	UpsertCard(card model.Card) error
	HardDeleteCard(projectSlug string, number int) error
	DeleteProject(projectSlug string) error
	ListCards(projectSlug string, includeDeleted bool) ([]model.CardSummary, error)
	RebuildFromMarkdown(projects []model.Project, cards []model.Card) error
}

type Publisher interface {
	Publish(event model.Event)
}

type RebuildResult struct {
	ProjectsRebuilt int
	CardsRebuilt    int
}

type Service struct {
	store      MarkdownStore
	projection Projection
	publisher  Publisher
	logger     *slog.Logger
}

func New(store MarkdownStore, projection Projection, publisher Publisher, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:      store,
		projection: projection,
		publisher:  publisher,
		logger:     logger,
	}
}

func (s *Service) CreateProject(name, localPath, remoteURL string) (model.Project, error) {
	project, err := s.store.CreateProject(name, localPath, remoteURL)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return model.Project{}, newError(CodeConflict, "project already exists", err)
		}
		return model.Project{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.projection.UpsertProject(project); err != nil {
		return model.Project{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("project created", "project", project.Slug)
	s.publish(model.Event{
		Type:      "project.created",
		Project:   project.Slug,
		Timestamp: time.Now().UTC(),
	})
	return project, nil
}

func (s *Service) ListProjects() ([]model.Project, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, newError(CodeInternal, "list projects failed", err)
	}
	return projects, nil
}

func (s *Service) DeleteProject(slug string) error {
	if err := s.store.DeleteProject(slug); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newError(CodeNotFound, "project not found", err)
		}
		return newError(CodeInternal, "delete project failed", err)
	}
	if err := s.projection.DeleteProject(slug); err != nil {
		return newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("project deleted", "project", slug)
	s.publish(model.Event{
		Type:      "project.deleted",
		Project:   slug,
		Timestamp: time.Now().UTC(),
	})
	return nil
}

func (s *Service) CreateCard(projectSlug, title, description, status, column string) (model.Card, error) {
	card, err := s.store.CreateCard(projectSlug, title, description, status, column)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeValidation, "project not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	project, err := s.store.GetProject(projectSlug)
	if err != nil {
		return model.Card{}, newError(CodeInternal, "load project failed", err)
	}
	if err := s.projection.UpsertProject(project); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card created", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number)
	s.publish(model.Event{
		Type:      "card.created",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) ListCards(projectSlug string, includeDeleted bool) ([]model.CardSummary, error) {
	cards, err := s.projection.ListCards(projectSlug, includeDeleted)
	if err != nil {
		return nil, newError(CodeInternal, "list cards failed", err)
	}
	return cards, nil
}

func (s *Service) GetCard(projectSlug string, number int) (model.Card, error) {
	card, err := s.store.GetCard(projectSlug, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeInternal, "get card failed", err)
	}
	return card, nil
}

func (s *Service) MoveCard(projectSlug string, number int, status, column string) (model.Card, error) {
	card, err := s.store.MoveCard(projectSlug, number, status, column)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card moved", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "status", card.Status, "column", card.Column)
	s.publish(model.Event{
		Type:      "card.moved",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) CommentCard(projectSlug string, number int, body string) (model.Card, error) {
	card, err := s.store.AddComment(projectSlug, number, body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card commented", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "comments_count", len(card.Comments))
	s.publish(model.Event{
		Type:      "card.commented",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) AppendDescription(projectSlug string, number int, body string) (model.Card, error) {
	card, err := s.store.AppendDescription(projectSlug, number, body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card description appended", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "description_entries", len(card.Description))
	s.publish(model.Event{
		Type:      "card.updated",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) DeleteCard(projectSlug string, number int, hard bool) (model.Card, error) {
	card, err := s.store.DeleteCard(projectSlug, number, hard)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}

	if hard {
		if err := s.projection.HardDeleteCard(projectSlug, number); err != nil {
			return model.Card{}, newError(CodeInternal, "projection sync failed", err)
		}
		s.logger.Info("card hard deleted", "project", projectSlug, "card_id", card.ID, "card_number", card.Number)
		s.publish(model.Event{
			Type:      "card.deleted_hard",
			Project:   projectSlug,
			CardID:    card.ID,
			CardNum:   card.Number,
			Timestamp: time.Now().UTC(),
		})
		return card, nil
	}

	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card soft deleted", "project", projectSlug, "card_id", card.ID, "card_number", card.Number)
	s.publish(model.Event{
		Type:      "card.deleted_soft",
		Project:   projectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) RebuildProjection() (RebuildResult, error) {
	projects, cards, err := s.store.Snapshot()
	if err != nil {
		return RebuildResult{}, newError(CodeInternal, "snapshot failed", err)
	}
	if err := s.projection.RebuildFromMarkdown(projects, cards); err != nil {
		return RebuildResult{}, newError(CodeInternal, "rebuild projection failed", err)
	}
	s.logger.Info("projection rebuilt", "projects_rebuilt", len(projects), "cards_rebuilt", len(cards))
	return RebuildResult{
		ProjectsRebuilt: len(projects),
		CardsRebuilt:    len(cards),
	}, nil
}

func (s *Service) publish(event model.Event) {
	if s.publisher == nil {
		return
	}
	event.Project = strings.TrimSpace(event.Project)
	s.publisher.Publish(event)
}
