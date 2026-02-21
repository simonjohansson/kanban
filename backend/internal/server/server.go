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
		OperationID:   "addTodo",
		Method:        http.MethodPost,
		Path:          "/projects/{project}/cards/{number}/todos",
		DefaultStatus: http.StatusCreated,
		Summary:       "Add card todo",
		Errors:        []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.addTodo)

	huma.Register(s.api, huma.Operation{
		OperationID: "listTodos",
		Method:      http.MethodGet,
		Path:        "/projects/{project}/cards/{number}/todos",
		Summary:     "List card todos",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.listTodos)

	huma.Register(s.api, huma.Operation{
		OperationID: "updateTodo",
		Method:      http.MethodPatch,
		Path:        "/projects/{project}/cards/{number}/todos/{todo_id}",
		Summary:     "Update card todo",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.updateTodo)

	huma.Register(s.api, huma.Operation{
		OperationID: "deleteTodo",
		Method:      http.MethodDelete,
		Path:        "/projects/{project}/cards/{number}/todos/{todo_id}",
		Summary:     "Delete card todo",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.deleteTodo)

	huma.Register(s.api, huma.Operation{
		OperationID:   "addAcceptanceCriterion",
		Method:        http.MethodPost,
		Path:          "/projects/{project}/cards/{number}/acceptance",
		DefaultStatus: http.StatusCreated,
		Summary:       "Add acceptance criterion",
		Errors:        []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.addAcceptanceCriterion)

	huma.Register(s.api, huma.Operation{
		OperationID: "listAcceptanceCriteria",
		Method:      http.MethodGet,
		Path:        "/projects/{project}/cards/{number}/acceptance",
		Summary:     "List acceptance criteria",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.listAcceptanceCriteria)

	huma.Register(s.api, huma.Operation{
		OperationID: "updateAcceptanceCriterion",
		Method:      http.MethodPatch,
		Path:        "/projects/{project}/cards/{number}/acceptance/{criterion_id}",
		Summary:     "Update acceptance criterion",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.updateAcceptanceCriterion)

	huma.Register(s.api, huma.Operation{
		OperationID: "deleteAcceptanceCriterion",
		Method:      http.MethodDelete,
		Path:        "/projects/{project}/cards/{number}/acceptance/{criterion_id}",
		Summary:     "Delete acceptance criterion",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.deleteAcceptanceCriterion)

	huma.Register(s.api, huma.Operation{
		OperationID: "setCardBranch",
		Method:      http.MethodPatch,
		Path:        "/projects/{project}/cards/{number}/branch",
		Summary:     "Set card branch metadata",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError},
	}, s.setCardBranch)

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
	ensureWebsocketEventSchemas(oapi)
	oapi.Paths["/ws"] = &huma.PathItem{
		Get: &huma.Operation{
			OperationID: "websocketEvents",
			Summary:     "Websocket event stream",
			Description: "Subscribe to project/card events. Optional project query param filters by project slug.",
			Responses: map[string]*huma.Response{
				"200": {
					Description: "Websocket event payload schema for generated clients.",
					Content: map[string]*huma.MediaType{
						"application/json": {
							Schema: &huma.Schema{Ref: "#/components/schemas/WebsocketEvent"},
						},
					},
				},
				"101": {
					Description: "Switching protocols to websocket",
					Content: map[string]*huma.MediaType{
						"application/json": {
							Schema: &huma.Schema{Ref: "#/components/schemas/WebsocketEvent"},
						},
					},
				},
			},
		},
	}
}

func ensureWebsocketEventSchemas(oapi *huma.OpenAPI) {
	if oapi.Components == nil {
		oapi.Components = &huma.Components{
			Schemas: huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer),
		}
	}
	if oapi.Components.Schemas == nil {
		oapi.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	}

	schemas := oapi.Components.Schemas.Map()
	schemas["WebsocketEventType"] = &huma.Schema{
		Type: "string",
		Enum: websocketEventKindEnumValues(),
	}
	schemas["WebsocketEvent"] = &huma.Schema{
		Type:     "object",
		Required: []string{"type", "project", "timestamp"},
		Properties: map[string]*huma.Schema{
			"type":        {Ref: "#/components/schemas/WebsocketEventType"},
			"project":     {Type: "string"},
			"card_id":     {Type: "string"},
			"card_number": {Type: "integer", Format: "int64"},
			"timestamp":   {Type: "string", Format: "date-time"},
		},
	}
}

func websocketEventKindEnumValues() []any {
	eventTypes := model.WebSocketEventTypes()
	values := make([]any, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		values = append(values, string(eventType))
	}
	return values
}
