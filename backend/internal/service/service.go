package service

import (
	"errors"
	"fmt"
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
	CreateCard(projectSlug, title, description, branch, status string) (model.Card, error)
	GetCard(projectSlug string, number int) (model.Card, error)
	MoveCard(projectSlug string, number int, status string) (model.Card, error)
	SetCardBranch(projectSlug string, number int, branch string) (model.Card, error)
	AddComment(projectSlug string, number int, body string) (model.Card, error)
	AppendDescription(projectSlug string, number int, body string) (model.Card, error)
	AddTodo(projectSlug string, number int, text string) (model.Todo, error)
	ListTodos(projectSlug string, number int) ([]model.Todo, error)
	SetTodoCompleted(projectSlug string, number int, todoID int, completed bool) (model.Todo, error)
	DeleteTodo(projectSlug string, number int, todoID int) (model.Todo, error)
	AddAcceptanceCriterion(projectSlug string, number int, text string) (model.AcceptanceCriterion, error)
	ListAcceptanceCriteria(projectSlug string, number int) ([]model.AcceptanceCriterion, error)
	SetAcceptanceCriterionCompleted(projectSlug string, number int, criterionID int, completed bool) (model.AcceptanceCriterion, error)
	DeleteAcceptanceCriterion(projectSlug string, number int, criterionID int) (model.AcceptanceCriterion, error)
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
		Type:      model.EventTypeProjectCreated,
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
		Type:      model.EventTypeProjectDeleted,
		Project:   slug,
		Timestamp: time.Now().UTC(),
	})
	return nil
}

func (s *Service) CreateCard(projectSlug, title, description, branch, status string) (model.Card, error) {
	card, err := s.store.CreateCard(projectSlug, title, description, branch, status)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeValidation, "project not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	card = normalizeCardDefaults(card)
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
		Type:      model.EventTypeCardCreated,
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) SetCardBranch(projectSlug string, number int, branch string) (model.Card, error) {
	card, err := s.store.SetCardBranch(projectSlug, number, branch)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	card = normalizeCardDefaults(card)
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card branch updated", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "branch", card.Branch)
	s.publish(model.Event{
		Type:      model.EventTypeCardBranchUpdated,
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
	return normalizeCardDefaults(card), nil
}

func (s *Service) MoveCard(projectSlug string, number int, status string) (model.Card, error) {
	card, err := s.store.MoveCard(projectSlug, number, status)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	card = normalizeCardDefaults(card)
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card moved", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "status", card.Status)
	s.publish(model.Event{
		Type:      model.EventTypeCardMoved,
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
	card = normalizeCardDefaults(card)
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card commented", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "comments_count", len(card.Comments))
	s.publish(model.Event{
		Type:      model.EventTypeCardCommented,
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
	card = normalizeCardDefaults(card)
	if err := s.projection.UpsertCard(card); err != nil {
		return model.Card{}, newError(CodeInternal, "projection sync failed", err)
	}
	s.logger.Info("card description appended", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "description_entries", len(card.Description))
	s.publish(model.Event{
		Type:      model.EventTypeCardUpdated,
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return card, nil
}

func (s *Service) AddTodo(projectSlug string, number int, text string) (model.Todo, error) {
	todo, err := s.store.AddTodo(projectSlug, number, text)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Todo{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Todo{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.Todo{}, err
	}
	s.logger.Info("card todo added", "project", projectSlug, "card_number", number, "todo_id", todo.ID)
	s.publish(model.Event{
		Type:      model.EventTypeCardTodoAdded,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return todo, nil
}

func (s *Service) ListTodos(projectSlug string, number int) ([]model.Todo, error) {
	todos, err := s.store.ListTodos(projectSlug, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, newError(CodeNotFound, "card not found", err)
		}
		return nil, newError(CodeInternal, "list todos failed", err)
	}
	if todos == nil {
		return []model.Todo{}, nil
	}
	return todos, nil
}

func (s *Service) SetTodoCompleted(projectSlug string, number int, todoID int, completed bool) (model.Todo, error) {
	todo, err := s.store.SetTodoCompleted(projectSlug, number, todoID, completed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Todo{}, newError(CodeNotFound, "todo not found", err)
		}
		return model.Todo{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.Todo{}, err
	}
	s.logger.Info("card todo updated", "project", projectSlug, "card_number", number, "todo_id", todoID, "completed", completed)
	s.publish(model.Event{
		Type:      model.EventTypeCardTodoUpdated,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return todo, nil
}

func (s *Service) DeleteTodo(projectSlug string, number int, todoID int) (model.Todo, error) {
	todo, err := s.store.DeleteTodo(projectSlug, number, todoID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Todo{}, newError(CodeNotFound, "todo not found", err)
		}
		return model.Todo{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.Todo{}, err
	}
	s.logger.Info("card todo deleted", "project", projectSlug, "card_number", number, "todo_id", todoID)
	s.publish(model.Event{
		Type:      model.EventTypeCardTodoDeleted,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return todo, nil
}

func (s *Service) AddAcceptanceCriterion(projectSlug string, number int, text string) (model.AcceptanceCriterion, error) {
	criterion, err := s.store.AddAcceptanceCriterion(projectSlug, number, text)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.AcceptanceCriterion{}, newError(CodeNotFound, "card not found", err)
		}
		return model.AcceptanceCriterion{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	s.logger.Info("card acceptance criterion added", "project", projectSlug, "card_number", number, "criterion_id", criterion.ID)
	s.publish(model.Event{
		Type:      model.EventTypeCardAcceptanceAdded,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return criterion, nil
}

func (s *Service) ListAcceptanceCriteria(projectSlug string, number int) ([]model.AcceptanceCriterion, error) {
	criteria, err := s.store.ListAcceptanceCriteria(projectSlug, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, newError(CodeNotFound, "card not found", err)
		}
		return nil, newError(CodeInternal, "list acceptance criteria failed", err)
	}
	if criteria == nil {
		return []model.AcceptanceCriterion{}, nil
	}
	return criteria, nil
}

func (s *Service) SetAcceptanceCriterionCompleted(projectSlug string, number int, criterionID int, completed bool) (model.AcceptanceCriterion, error) {
	criterion, err := s.store.SetAcceptanceCriterionCompleted(projectSlug, number, criterionID, completed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.AcceptanceCriterion{}, newError(CodeNotFound, "acceptance criterion not found", err)
		}
		return model.AcceptanceCriterion{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	s.logger.Info("card acceptance criterion updated", "project", projectSlug, "card_number", number, "criterion_id", criterionID, "completed", completed)
	s.publish(model.Event{
		Type:      model.EventTypeCardAcceptanceUpdated,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return criterion, nil
}

func (s *Service) DeleteAcceptanceCriterion(projectSlug string, number int, criterionID int) (model.AcceptanceCriterion, error) {
	criterion, err := s.store.DeleteAcceptanceCriterion(projectSlug, number, criterionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.AcceptanceCriterion{}, newError(CodeNotFound, "acceptance criterion not found", err)
		}
		return model.AcceptanceCriterion{}, newError(CodeValidation, err.Error(), err)
	}
	if err := s.syncCardProjection(projectSlug, number); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	s.logger.Info("card acceptance criterion deleted", "project", projectSlug, "card_number", number, "criterion_id", criterionID)
	s.publish(model.Event{
		Type:      model.EventTypeCardAcceptanceDeleted,
		Project:   projectSlug,
		CardID:    fmt.Sprintf("%s/card-%d", projectSlug, number),
		CardNum:   number,
		Timestamp: time.Now().UTC(),
	})
	return criterion, nil
}

func (s *Service) DeleteCard(projectSlug string, number int, hard bool) (model.Card, error) {
	card, err := s.store.DeleteCard(projectSlug, number, hard)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Card{}, newError(CodeNotFound, "card not found", err)
		}
		return model.Card{}, newError(CodeValidation, err.Error(), err)
	}
	card = normalizeCardDefaults(card)

	if hard {
		if err := s.projection.HardDeleteCard(projectSlug, number); err != nil {
			return model.Card{}, newError(CodeInternal, "projection sync failed", err)
		}
		s.logger.Info("card hard deleted", "project", projectSlug, "card_id", card.ID, "card_number", card.Number)
		s.publish(model.Event{
			Type:      model.EventTypeCardDeletedHard,
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
		Type:      model.EventTypeCardDeletedSoft,
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

func normalizeCardDefaults(card model.Card) model.Card {
	if card.Description == nil {
		card.Description = []model.TextEvent{}
	}
	if card.Comments == nil {
		card.Comments = []model.TextEvent{}
	}
	if card.History == nil {
		card.History = []model.HistoryEvent{}
	}
	if card.Todos == nil {
		card.Todos = []model.Todo{}
	}
	if card.AcceptanceCriteria == nil {
		card.AcceptanceCriteria = []model.AcceptanceCriterion{}
	}
	if card.NextTodoID <= 0 {
		card.NextTodoID = nextTodoID(card.Todos)
	}
	if card.NextAcceptanceCriterionID <= 0 {
		card.NextAcceptanceCriterionID = nextAcceptanceCriterionID(card.AcceptanceCriteria)
	}
	return card
}

func nextTodoID(todos []model.Todo) int {
	maxID := 0
	for _, todo := range todos {
		if todo.ID > maxID {
			maxID = todo.ID
		}
	}
	return maxID + 1
}

func nextAcceptanceCriterionID(criteria []model.AcceptanceCriterion) int {
	maxID := 0
	for _, criterion := range criteria {
		if criterion.ID > maxID {
			maxID = criterion.ID
		}
	}
	return maxID + 1
}

func (s *Service) syncCardProjection(projectSlug string, number int) error {
	card, err := s.store.GetCard(projectSlug, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newError(CodeNotFound, "card not found", err)
		}
		return newError(CodeInternal, "get card failed", err)
	}
	card = normalizeCardDefaults(card)
	if err := s.projection.UpsertCard(card); err != nil {
		return newError(CodeInternal, "projection sync failed", err)
	}
	return nil
}
