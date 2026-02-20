package server

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
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
	if err := os.MkdirAll(filepath.Dir(opts.SQLitePath), 0o755); err != nil {
		return nil, err
	}
	if err := os.Remove(opts.SQLitePath); err != nil && !errors.Is(err, os.ErrNotExist) {
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

	if result, err := s.service.RebuildProjection(); err != nil {
		_ = projection.Close()
		return nil, err
	} else {
		s.logger.Info("projection rebuilt on startup", "projects_rebuilt", result.ProjectsRebuilt, "cards_rebuilt", result.CardsRebuilt)
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
		OperationID: "getClientConfig",
		Method:      http.MethodGet,
		Path:        "/client-config",
		Summary:     "Get client runtime config",
		Errors:      []int{},
	}, s.clientConfig)

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
