package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/simonjohansson/kanban/backend/internal/store"
)

type Options struct {
	DataDir    string
	SQLitePath string
	Logger     *slog.Logger
}

type Server struct {
	store      *store.MarkdownStore
	projection *store.SQLiteProjection
	hub        *hub
	logger     *slog.Logger
	router     *chi.Mux
	api        huma.API
}

func New(opts Options) (*Server, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	markdownStore, err := store.NewMarkdownStore(opts.DataDir)
	if err != nil {
		return nil, err
	}
	projection, err := store.NewSQLiteProjection(opts.SQLitePath)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	s := &Server{
		store:      markdownStore,
		projection: projection,
		hub:        newHub(),
		logger:     logger,
		router:     router,
	}
	s.routes()
	s.logger.Info("server initialized", "data_dir", opts.DataDir, "sqlite_path", opts.SQLitePath)
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) OpenAPI() *huma.OpenAPI {
	return s.api.OpenAPI()
}

func (s *Server) Close() error {
	s.hub.Close()
	return s.projection.Close()
}

func (s *Server) routes() {
	s.router.Use(s.requestLoggingMiddleware)

	config := huma.DefaultConfig("Kanban Backend API", "1.0.0")
	config.OpenAPIPath = "/openapi"
	config.DocsPath = ""

	s.api = humachi.New(s.router, config)
	s.registerOperations()
	s.registerWebSocketOperationDocs()

	// Websocket upgrade endpoint remains a native HTTP handler.
	s.router.Get("/ws", s.hub.ServeWS)
}

func (s *Server) registerOperations() {
	huma.Get(s.api, "/health", s.health)

	huma.Register(s.api, huma.Operation{
		OperationID:   "createProject",
		Method:        http.MethodPost,
		Path:          "/projects",
		DefaultStatus: http.StatusCreated,
		Summary:       "Create project",
		Errors:        []int{http.StatusBadRequest, http.StatusConflict, http.StatusInternalServerError},
	}, s.createProject)

	huma.Register(s.api, huma.Operation{
		OperationID: "listProjects",
		Method:      http.MethodGet,
		Path:        "/projects",
		Summary:     "List projects",
		Errors:      []int{http.StatusInternalServerError},
	}, s.listProjects)

	huma.Register(s.api, huma.Operation{
		OperationID:   "createCard",
		Method:        http.MethodPost,
		Path:          "/projects/{project}/cards",
		DefaultStatus: http.StatusCreated,
		Summary:       "Create card",
		Errors:        []int{http.StatusBadRequest, http.StatusInternalServerError},
	}, s.createCard)

	huma.Register(s.api, huma.Operation{
		OperationID: "listCards",
		Method:      http.MethodGet,
		Path:        "/projects/{project}/cards",
		Summary:     "List cards",
		Errors:      []int{http.StatusInternalServerError},
	}, s.listCards)

	huma.Register(s.api, huma.Operation{
		OperationID: "getCard",
		Method:      http.MethodGet,
		Path:        "/projects/{project}/cards/{number}",
		Summary:     "Get card",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.getCard)

	huma.Register(s.api, huma.Operation{
		OperationID: "moveCard",
		Method:      http.MethodPatch,
		Path:        "/projects/{project}/cards/{number}/move",
		Summary:     "Move card",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.moveCard)

	huma.Register(s.api, huma.Operation{
		OperationID: "commentCard",
		Method:      http.MethodPost,
		Path:        "/projects/{project}/cards/{number}/comments",
		Summary:     "Append card comment",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.commentCard)

	huma.Register(s.api, huma.Operation{
		OperationID: "appendDescription",
		Method:      http.MethodPatch,
		Path:        "/projects/{project}/cards/{number}/description",
		Summary:     "Append card description entry",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.appendDescription)

	huma.Register(s.api, huma.Operation{
		OperationID: "deleteCard",
		Method:      http.MethodDelete,
		Path:        "/projects/{project}/cards/{number}",
		Summary:     "Delete card",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.deleteCard)

	huma.Register(s.api, huma.Operation{
		OperationID: "rebuildProjection",
		Method:      http.MethodPost,
		Path:        "/admin/rebuild",
		Summary:     "Rebuild SQLite projection from markdown",
		Errors:      []int{http.StatusInternalServerError},
	}, s.rebuildProjection)
}

func (s *Server) registerWebSocketOperationDocs() {
	oapi := s.api.OpenAPI()
	if oapi.Paths == nil {
		oapi.Paths = map[string]*huma.PathItem{}
	}
	oapi.Paths["/ws"] = &huma.PathItem{
		Get: &huma.Operation{
			OperationID: "websocketEvents",
			Summary:     "Websocket event stream",
			Description: "Subscribe to project/card events. Optional project query param filters by project slug.",
			Responses: map[string]*huma.Response{
				"101": {Description: "Switching protocols to websocket"},
			},
		},
	}
}

type healthOutput struct {
	Body struct {
		Ok bool `json:"ok"`
	}
}

func (s *Server) health(_ context.Context, _ *struct{}) (*healthOutput, error) {
	out := &healthOutput{}
	out.Body.Ok = true
	return out, nil
}

type createProjectRequest struct {
	Name      string  `json:"name"`
	LocalPath *string `json:"local_path,omitempty"`
	RemoteURL *string `json:"remote_url,omitempty"`
}

type createProjectInput struct {
	Body createProjectRequest
}

type createProjectOutput struct {
	Body model.Project
}

func (s *Server) createProject(_ context.Context, input *createProjectInput) (*createProjectOutput, error) {
	project, err := s.store.CreateProject(input.Body.Name, stringOrEmpty(input.Body.LocalPath), stringOrEmpty(input.Body.RemoteURL))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, huma.Error409Conflict("project already exists")
		}
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := s.projection.UpsertProject(project); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	s.logger.Info("project created", "project", project.Slug)
	s.publishEvent(model.Event{Type: "project.created", Project: project.Slug, Timestamp: time.Now().UTC()})

	out := &createProjectOutput{Body: project}
	return out, nil
}

type listProjectsOutput struct {
	Body struct {
		Projects []model.Project `json:"projects"`
	}
}

func (s *Server) listProjects(_ context.Context, _ *struct{}) (*listProjectsOutput, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	out := &listProjectsOutput{}
	out.Body.Projects = projects
	return out, nil
}

type createCardRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	Column      *string `json:"column,omitempty"`
}

type createCardInput struct {
	Project string `path:"project"`
	Body    createCardRequest
}

type createCardOutput struct {
	Body model.Card
}

func (s *Server) createCard(_ context.Context, input *createCardInput) (*createCardOutput, error) {
	card, err := s.store.CreateCard(input.Project, input.Body.Title, stringOrEmpty(input.Body.Description), input.Body.Status, stringOrEmpty(input.Body.Column))
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	project, err := s.store.GetProject(input.Project)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if err := s.projection.UpsertProject(project); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	s.logger.Info("card created", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number)
	s.publishEvent(model.Event{
		Type:      "card.created",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})

	out := &createCardOutput{Body: card}
	return out, nil
}

type listCardsInput struct {
	Project        string `path:"project"`
	IncludeDeleted bool   `query:"include_deleted"`
}

type listCardsOutput struct {
	Body struct {
		Cards []model.CardSummary `json:"cards"`
	}
}

func (s *Server) listCards(_ context.Context, input *listCardsInput) (*listCardsOutput, error) {
	cards, err := s.projection.ListCards(input.Project, input.IncludeDeleted)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	out := &listCardsOutput{}
	out.Body.Cards = cards
	return out, nil
}

type cardPathInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
}

type getCardOutput struct {
	Body model.Card
}

func (s *Server) getCard(_ context.Context, input *cardPathInput) (*getCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.store.GetCard(input.Project, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, huma.Error404NotFound("card not found")
		}
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &getCardOutput{Body: card}, nil
}

type moveCardRequest struct {
	Status string  `json:"status"`
	Column *string `json:"column,omitempty"`
}

type moveCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    moveCardRequest
}

type moveCardOutput struct {
	Body model.Card
}

func (s *Server) moveCard(_ context.Context, input *moveCardInput) (*moveCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.store.MoveCard(input.Project, number, input.Body.Status, stringOrEmpty(input.Body.Column))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, huma.Error404NotFound("card not found")
		}
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	s.logger.Info("card moved", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "status", card.Status, "column", card.Column)
	s.publishEvent(model.Event{
		Type:      "card.moved",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return &moveCardOutput{Body: card}, nil
}

type textBodyRequest struct {
	Body string `json:"body"`
}

type commentCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    textBodyRequest
}

type commentCardOutput struct {
	Body model.Card
}

func (s *Server) commentCard(_ context.Context, input *commentCardInput) (*commentCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.store.AddComment(input.Project, number, input.Body.Body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, huma.Error404NotFound("card not found")
		}
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	s.logger.Info("card commented", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "comments_count", len(card.Comments))
	s.publishEvent(model.Event{
		Type:      "card.commented",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return &commentCardOutput{Body: card}, nil
}

type appendDescriptionInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Body    textBodyRequest
}

type appendDescriptionOutput struct {
	Body model.Card
}

func (s *Server) appendDescription(_ context.Context, input *appendDescriptionInput) (*appendDescriptionOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.store.AppendDescription(input.Project, number, input.Body.Body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, huma.Error404NotFound("card not found")
		}
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err := s.projection.UpsertCard(card); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	s.logger.Info("card description appended", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "description_entries", len(card.Description))
	s.publishEvent(model.Event{
		Type:      "card.updated",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	return &appendDescriptionOutput{Body: card}, nil
}

type deleteCardInput struct {
	Project string `path:"project"`
	Number  int    `path:"number"`
	Hard    bool   `query:"hard"`
}

type deleteCardOutput struct {
	Body model.Card
}

func (s *Server) deleteCard(_ context.Context, input *deleteCardInput) (*deleteCardOutput, error) {
	number, err := normalizeCardNumber(input.Number)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	card, err := s.store.DeleteCard(input.Project, number, input.Hard)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, huma.Error404NotFound("card not found")
		}
		return nil, huma.Error400BadRequest(err.Error())
	}

	if input.Hard {
		if err := s.projection.HardDeleteCard(input.Project, number); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		s.logger.Info("card hard deleted", "project", input.Project, "card_id", card.ID, "card_number", card.Number)
		s.publishEvent(model.Event{
			Type:      "card.deleted_hard",
			Project:   input.Project,
			CardID:    card.ID,
			CardNum:   card.Number,
			Timestamp: time.Now().UTC(),
		})
	} else {
		if err := s.projection.UpsertCard(card); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		s.logger.Info("card soft deleted", "project", input.Project, "card_id", card.ID, "card_number", card.Number)
		s.publishEvent(model.Event{
			Type:      "card.deleted_soft",
			Project:   input.Project,
			CardID:    card.ID,
			CardNum:   card.Number,
			Timestamp: time.Now().UTC(),
		})
	}

	return &deleteCardOutput{Body: card}, nil
}

type rebuildProjectionOutput struct {
	Body struct {
		ProjectsRebuilt int `json:"projects_rebuilt"`
		CardsRebuilt    int `json:"cards_rebuilt"`
	}
}

func (s *Server) rebuildProjection(_ context.Context, _ *struct{}) (*rebuildProjectionOutput, error) {
	projects, cards, err := s.store.Snapshot()
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if err := s.projection.RebuildFromMarkdown(projects, cards); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	out := &rebuildProjectionOutput{}
	out.Body.ProjectsRebuilt = len(projects)
	out.Body.CardsRebuilt = len(cards)
	s.logger.Info("projection rebuilt", "projects_rebuilt", len(projects), "cards_rebuilt", len(cards))
	return out, nil
}

func normalizeCardNumber(number int) (int, error) {
	if number <= 0 {
		return 0, fmt.Errorf("invalid card number")
	}
	return number, nil
}

func stringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func (s *Server) publishEvent(event model.Event) {
	// Ensure project filter consistency on websocket subscriptions.
	event.Project = strings.TrimSpace(event.Project)
	s.hub.Publish(event)
}
