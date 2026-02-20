package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/simonjohansson/kanban/backend/internal/service"
	"github.com/simonjohansson/kanban/backend/internal/store"
)

type Options struct {
	DataDir    string
	SQLitePath string
	Logger     *slog.Logger
}

type Server struct {
	service    *service.Service
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
	hub := newHub()

	router := chi.NewRouter()
	s := &Server{
		service:    service.New(markdownStore, projection, hub, logger),
		projection: projection,
		hub:        hub,
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
	s.router.Get("/*", s.frontendHandler())
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
		OperationID: "deleteProject",
		Method:      http.MethodDelete,
		Path:        "/projects/{project}",
		Summary:     "Delete project",
		Errors:      []int{http.StatusNotFound, http.StatusInternalServerError},
	}, s.deleteProject)

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
	project, err := s.service.CreateProject(input.Body.Name, stringOrEmpty(input.Body.LocalPath), stringOrEmpty(input.Body.RemoteURL))
	if err != nil {
		return nil, toHumaError(err)
	}

	out := &createProjectOutput{Body: project}
	return out, nil
}

type listProjectsOutput struct {
	Body struct {
		Projects []model.Project `json:"projects"`
	}
}

func (s *Server) listProjects(_ context.Context, _ *struct{}) (*listProjectsOutput, error) {
	projects, err := s.service.ListProjects()
	if err != nil {
		return nil, toHumaError(err)
	}
	out := &listProjectsOutput{}
	out.Body.Projects = projects
	return out, nil
}

type deleteProjectInput struct {
	Project string `path:"project"`
}

type deleteProjectOutput struct {
	Body struct {
		Project string `json:"project"`
		Deleted bool   `json:"deleted"`
	}
}

func (s *Server) deleteProject(_ context.Context, input *deleteProjectInput) (*deleteProjectOutput, error) {
	if err := s.service.DeleteProject(input.Project); err != nil {
		return nil, toHumaError(err)
	}

	out := &deleteProjectOutput{}
	out.Body.Project = input.Project
	out.Body.Deleted = true
	return out, nil
}

type createCardRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
}

type createCardInput struct {
	Project string `path:"project"`
	Body    createCardRequest
}

type createCardOutput struct {
	Body model.Card
}

func (s *Server) createCard(_ context.Context, input *createCardInput) (*createCardOutput, error) {
	card, err := s.service.CreateCard(input.Project, input.Body.Title, stringOrEmpty(input.Body.Description), input.Body.Status)
	if err != nil {
		return nil, toHumaError(err)
	}

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
	cards, err := s.service.ListCards(input.Project, input.IncludeDeleted)
	if err != nil {
		return nil, toHumaError(err)
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
	card, err := s.service.GetCard(input.Project, number)
	if err != nil {
		return nil, toHumaError(err)
	}
	return &getCardOutput{Body: card}, nil
}

type moveCardRequest struct {
	Status string  `json:"status"`
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
	card, err := s.service.MoveCard(input.Project, number, input.Body.Status)
	if err != nil {
		return nil, toHumaError(err)
	}
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
	card, err := s.service.CommentCard(input.Project, number, input.Body.Body)
	if err != nil {
		return nil, toHumaError(err)
	}
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
	card, err := s.service.AppendDescription(input.Project, number, input.Body.Body)
	if err != nil {
		return nil, toHumaError(err)
	}
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
	card, err := s.service.DeleteCard(input.Project, number, input.Hard)
	if err != nil {
		return nil, toHumaError(err)
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
	result, err := s.service.RebuildProjection()
	if err != nil {
		return nil, toHumaError(err)
	}

	out := &rebuildProjectionOutput{}
	out.Body.ProjectsRebuilt = result.ProjectsRebuilt
	out.Body.CardsRebuilt = result.CardsRebuilt
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

func toHumaError(err error) error {
	code := service.CodeOf(err)
	msg := service.MessageOf(err)
	switch code {
	case service.CodeConflict:
		return huma.Error409Conflict(msg)
	case service.CodeNotFound:
		return huma.Error404NotFound(msg)
	case service.CodeValidation:
		return huma.Error400BadRequest(msg)
	default:
		return huma.Error500InternalServerError(msg)
	}
}
