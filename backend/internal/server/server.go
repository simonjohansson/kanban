package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/simonjohansson/kanban/backend/api"
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
	s := &Server{store: markdownStore, projection: projection, hub: newHub(), logger: logger}
	s.router = chi.NewRouter()
	s.routes()
	s.logger.Info("server initialized", "data_dir", opts.DataDir, "sqlite_path", opts.SQLitePath)
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) Close() error {
	s.hub.Close()
	return s.projection.Close()
}

func (s *Server) routes() {
	s.router.Use(s.requestLoggingMiddleware)
	s.router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	s.router.Get("/openapi.yaml", s.openAPIYAML)
	s.router.Post("/projects", s.createProject)
	s.router.Get("/projects", s.listProjects)
	s.router.Get("/ws", s.hub.ServeWS)
	s.router.Post("/admin/rebuild", s.rebuildProjection)
	s.router.Post("/projects/{project}/cards", s.createCard)
	s.router.Get("/projects/{project}/cards", s.listCards)
	s.router.Get("/projects/{project}/cards/{number}", s.getCard)
	s.router.Patch("/projects/{project}/cards/{number}/move", s.moveCard)
	s.router.Post("/projects/{project}/cards/{number}/comments", s.commentCard)
	s.router.Patch("/projects/{project}/cards/{number}/description", s.appendDescription)
	s.router.Delete("/projects/{project}/cards/{number}", s.deleteCard)
}

type createProjectRequest struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	RemoteURL string `json:"remote_url"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	project, err := s.store.CreateProject(req.Name, req.LocalPath, req.RemoteURL)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			writeError(w, http.StatusConflict, "project already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.projection.UpsertProject(project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("project created", "project", project.Slug)
	s.publishEvent(model.Event{Type: "project.created", Project: project.Slug, Timestamp: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

type createCardRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Column      string `json:"column"`
}

func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	var req createCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := s.store.CreateCard(projectSlug, req.Title, req.Description, req.Status, req.Column)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.store.GetProject(projectSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.projection.UpsertProject(project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.projection.UpsertCard(card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("card created", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number)
	s.publishEvent(model.Event{
		Type:      "card.created",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	includeDeleted := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_deleted")), "true")
	cards, err := s.projection.ListCards(projectSlug, includeDeleted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	number, err := cardNumberParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	card, err := s.store.GetCard(projectSlug, number)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, card)
}

type moveCardRequest struct {
	Status string `json:"status"`
	Column string `json:"column"`
}

func (s *Server) moveCard(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	number, err := cardNumberParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req moveCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := s.store.MoveCard(projectSlug, number, req.Status, req.Column)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.projection.UpsertCard(card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("card moved", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "status", card.Status, "column", card.Column)
	s.publishEvent(model.Event{
		Type:      "card.moved",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, card)
}

type textBodyRequest struct {
	Body string `json:"body"`
}

func (s *Server) commentCard(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	number, err := cardNumberParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req textBodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := s.store.AddComment(projectSlug, number, req.Body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.projection.UpsertCard(card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("card commented", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "comments_count", len(card.Comments))
	s.publishEvent(model.Event{
		Type:      "card.commented",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) appendDescription(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	number, err := cardNumberParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req textBodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := s.store.AppendDescription(projectSlug, number, req.Body)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.projection.UpsertCard(card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("card description appended", "project", card.ProjectSlug, "card_id", card.ID, "card_number", card.Number, "description_entries", len(card.Description))
	s.publishEvent(model.Event{
		Type:      "card.updated",
		Project:   card.ProjectSlug,
		CardID:    card.ID,
		CardNum:   card.Number,
		Timestamp: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	projectSlug := chi.URLParam(r, "project")
	number, err := cardNumberParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hardDelete := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("hard")), "true")
	card, err := s.store.DeleteCard(projectSlug, number, hardDelete)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if hardDelete {
		if err := s.projection.HardDeleteCard(projectSlug, number); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.logger.Info("card hard deleted", "project", projectSlug, "card_id", card.ID, "card_number", card.Number)
		s.publishEvent(model.Event{
			Type:      "card.deleted_hard",
			Project:   projectSlug,
			CardID:    card.ID,
			CardNum:   card.Number,
			Timestamp: time.Now().UTC(),
		})
	} else if err := s.projection.UpsertCard(card); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else {
		s.logger.Info("card soft deleted", "project", projectSlug, "card_id", card.ID, "card_number", card.Number)
		s.publishEvent(model.Event{
			Type:      "card.deleted_soft",
			Project:   projectSlug,
			CardID:    card.ID,
			CardNum:   card.Number,
			Timestamp: time.Now().UTC(),
		})
	}
	writeJSON(w, http.StatusOK, card)
}

func cardNumberParam(r *http.Request) (int, error) {
	numberRaw := chi.URLParam(r, "number")
	number, err := strconv.Atoi(numberRaw)
	if err != nil || number <= 0 {
		return 0, errors.New("invalid card number")
	}
	return number, nil
}

func (s *Server) publishEvent(event model.Event) {
	s.hub.Publish(event)
}

func (s *Server) rebuildProjection(w http.ResponseWriter, _ *http.Request) {
	projects, cards, err := s.store.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.projection.RebuildFromMarkdown(projects, cards); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects_rebuilt": len(projects),
		"cards_rebuilt":    len(cards),
	})
	s.logger.Info("projection rebuilt", "projects_rebuilt", len(projects), "cards_rebuilt", len(cards))
}

func (s *Server) openAPIYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(api.OpenAPIYAML())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
