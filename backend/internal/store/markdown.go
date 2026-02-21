package store

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/model"
	"gopkg.in/yaml.v3"
)

type MarkdownStore struct {
	dataDir     string
	projectsDir string
	mu          sync.RWMutex
}

var renameFile = os.Rename

func NewMarkdownStore(dataDir string) (*MarkdownStore, error) {
	projectsDir := filepath.Join(dataDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		return nil, err
	}
	return &MarkdownStore{dataDir: dataDir, projectsDir: projectsDir}, nil
}

type projectFrontmatter struct {
	Name        string    `yaml:"name"`
	Slug        string    `yaml:"slug"`
	LocalPath   string    `yaml:"local_path,omitempty"`
	RemoteURL   string    `yaml:"remote_url,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"`
	NextCardSeq int       `yaml:"next_card_seq"`
}

type cardFrontmatter struct {
	ID                        string    `yaml:"id"`
	ProjectSlug               string    `yaml:"project"`
	Number                    int       `yaml:"number"`
	Title                     string    `yaml:"title"`
	Branch                    string    `yaml:"branch,omitempty"`
	Status                    string    `yaml:"status"`
	Column                    string    `yaml:"column,omitempty"`
	Deleted                   bool      `yaml:"deleted"`
	CreatedAt                 time.Time `yaml:"created_at"`
	UpdatedAt                 time.Time `yaml:"updated_at"`
	NextTodoID                int       `yaml:"next_todo_id,omitempty"`
	NextAcceptanceCriterionID int       `yaml:"next_acceptance_criterion_id,omitempty"`
}

func (s *MarkdownStore) CreateProject(name, localPath, remoteURL string) (model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return model.Project{}, errors.New("name is required")
	}
	slug := Slugify(name)
	projectDir := s.projectDir(slug)
	if _, err := os.Stat(projectDir); err == nil {
		return model.Project{}, os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return model.Project{}, err
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return model.Project{}, err
	}
	now := time.Now().UTC()
	project := model.Project{
		Name:        name,
		Slug:        slug,
		LocalPath:   strings.TrimSpace(localPath),
		RemoteURL:   strings.TrimSpace(remoteURL),
		CreatedAt:   now,
		UpdatedAt:   now,
		NextCardSeq: 1,
	}
	if err := s.writeProject(project); err != nil {
		return model.Project{}, err
	}
	return project, nil
}

func (s *MarkdownStore) ListProjects() ([]model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dirs, err := os.ReadDir(s.projectsDir)
	if err != nil {
		return nil, err
	}
	projects := make([]model.Project, 0, len(dirs))
	for _, entry := range dirs {
		if !entry.IsDir() {
			continue
		}
		project, err := s.loadProject(entry.Name())
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Slug < projects[j].Slug })
	return projects, nil
}

func (s *MarkdownStore) GetProject(slug string) (model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadProject(slug)
}

func (s *MarkdownStore) DeleteProject(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	projectDir := s.projectDir(slug)
	if _, err := os.Stat(projectDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.ErrNotExist
		}
		return err
	}
	return os.RemoveAll(projectDir)
}

func (s *MarkdownStore) CreateCard(projectSlug, title, description, branch, status string) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	title = strings.TrimSpace(title)
	if title == "" {
		return model.Card{}, errors.New("title is required")
	}
	if err := validateStatus(status); err != nil {
		return model.Card{}, err
	}
	branch = strings.TrimSpace(branch)
	if err := validateBranchName(branch); err != nil {
		return model.Card{}, err
	}
	project, err := s.loadProject(projectSlug)
	if err != nil {
		return model.Card{}, err
	}
	now := time.Now().UTC()
	number := project.NextCardSeq
	project.NextCardSeq++
	project.UpdatedAt = now

	card := model.Card{
		ID:                        fmt.Sprintf("%s/card-%d", projectSlug, number),
		ProjectSlug:               projectSlug,
		Number:                    number,
		Title:                     title,
		Branch:                    branch,
		Status:                    status,
		Deleted:                   false,
		CreatedAt:                 now,
		UpdatedAt:                 now,
		NextTodoID:                1,
		NextAcceptanceCriterionID: 1,
	}
	if strings.TrimSpace(description) != "" {
		card.Description = append(card.Description, model.TextEvent{Timestamp: now, Body: strings.TrimSpace(description)})
	}
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.created",
		Details:   fmt.Sprintf("status=%s", status),
	})

	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	if err := s.writeProject(project); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) GetCard(projectSlug string, number int) (model.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getCardUnlocked(projectSlug, number)
}

func (s *MarkdownStore) getCardUnlocked(projectSlug string, number int) (model.Card, error) {
	data, err := os.ReadFile(s.cardPath(projectSlug, number))
	if err != nil {
		return model.Card{}, err
	}
	card, err := parseCard(data)
	if err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) AppendDescription(projectSlug string, number int, body string) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body = strings.TrimSpace(body)
	if body == "" {
		return model.Card{}, errors.New("description body is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Card{}, err
	}
	now := time.Now().UTC()
	card.Description = append(card.Description, model.TextEvent{Timestamp: now, Body: body})
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{Timestamp: now, Type: "card.updated", Details: "description appended"})
	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) AddComment(projectSlug string, number int, body string) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body = strings.TrimSpace(body)
	if body == "" {
		return model.Card{}, errors.New("comment body is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Card{}, err
	}
	now := time.Now().UTC()
	card.Comments = append(card.Comments, model.TextEvent{Timestamp: now, Body: body})
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{Timestamp: now, Type: "card.commented", Details: "comment appended"})
	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) AddTodo(projectSlug string, number int, text string) (model.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" {
		return model.Todo{}, errors.New("todo text is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Todo{}, err
	}

	now := time.Now().UTC()
	todoID := card.NextTodoID
	if todoID <= 0 {
		todoID = nextTodoID(card.Todos)
	}
	todo := model.Todo{ID: todoID, Text: text, Completed: false}
	card.Todos = append(card.Todos, todo)
	card.NextTodoID = todoID + 1
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.todo.added",
		Details:   fmt.Sprintf("todo_id=%d", todo.ID),
	})
	if err := s.writeCard(card); err != nil {
		return model.Todo{}, err
	}
	return todo, nil
}

func (s *MarkdownStore) ListTodos(projectSlug string, number int) ([]model.Todo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return nil, err
	}
	if len(card.Todos) == 0 {
		return []model.Todo{}, nil
	}
	out := make([]model.Todo, len(card.Todos))
	copy(out, card.Todos)
	return out, nil
}

func (s *MarkdownStore) SetTodoCompleted(projectSlug string, number int, todoID int, completed bool) (model.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if todoID <= 0 {
		return model.Todo{}, errors.New("todo id is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Todo{}, err
	}

	idx := indexOfTodo(card.Todos, todoID)
	if idx < 0 {
		return model.Todo{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	card.Todos[idx].Completed = completed
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.todo.updated",
		Details:   fmt.Sprintf("todo_id=%d completed=%t", todoID, completed),
	})
	if err := s.writeCard(card); err != nil {
		return model.Todo{}, err
	}
	return card.Todos[idx], nil
}

func (s *MarkdownStore) DeleteTodo(projectSlug string, number int, todoID int) (model.Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if todoID <= 0 {
		return model.Todo{}, errors.New("todo id is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Todo{}, err
	}

	idx := indexOfTodo(card.Todos, todoID)
	if idx < 0 {
		return model.Todo{}, os.ErrNotExist
	}
	removed := card.Todos[idx]
	card.Todos = append(card.Todos[:idx], card.Todos[idx+1:]...)
	now := time.Now().UTC()
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.todo.deleted",
		Details:   fmt.Sprintf("todo_id=%d", todoID),
	})
	if err := s.writeCard(card); err != nil {
		return model.Todo{}, err
	}
	return removed, nil
}

func (s *MarkdownStore) AddAcceptanceCriterion(projectSlug string, number int, text string) (model.AcceptanceCriterion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" {
		return model.AcceptanceCriterion{}, errors.New("acceptance criterion text is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.AcceptanceCriterion{}, err
	}

	now := time.Now().UTC()
	criterionID := card.NextAcceptanceCriterionID
	if criterionID <= 0 {
		criterionID = nextAcceptanceCriterionID(card.AcceptanceCriteria)
	}
	criterion := model.AcceptanceCriterion{ID: criterionID, Text: text, Completed: false}
	card.AcceptanceCriteria = append(card.AcceptanceCriteria, criterion)
	card.NextAcceptanceCriterionID = criterionID + 1
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.acceptance.added",
		Details:   fmt.Sprintf("criterion_id=%d", criterion.ID),
	})
	if err := s.writeCard(card); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	return criterion, nil
}

func (s *MarkdownStore) ListAcceptanceCriteria(projectSlug string, number int) ([]model.AcceptanceCriterion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return nil, err
	}
	if len(card.AcceptanceCriteria) == 0 {
		return []model.AcceptanceCriterion{}, nil
	}
	out := make([]model.AcceptanceCriterion, len(card.AcceptanceCriteria))
	copy(out, card.AcceptanceCriteria)
	return out, nil
}

func (s *MarkdownStore) SetAcceptanceCriterionCompleted(projectSlug string, number int, criterionID int, completed bool) (model.AcceptanceCriterion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if criterionID <= 0 {
		return model.AcceptanceCriterion{}, errors.New("criterion id is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.AcceptanceCriterion{}, err
	}

	idx := indexOfAcceptanceCriterion(card.AcceptanceCriteria, criterionID)
	if idx < 0 {
		return model.AcceptanceCriterion{}, os.ErrNotExist
	}
	now := time.Now().UTC()
	card.AcceptanceCriteria[idx].Completed = completed
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.acceptance.updated",
		Details:   fmt.Sprintf("criterion_id=%d completed=%t", criterionID, completed),
	})
	if err := s.writeCard(card); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	return card.AcceptanceCriteria[idx], nil
}

func (s *MarkdownStore) DeleteAcceptanceCriterion(projectSlug string, number int, criterionID int) (model.AcceptanceCriterion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if criterionID <= 0 {
		return model.AcceptanceCriterion{}, errors.New("criterion id is required")
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.AcceptanceCriterion{}, err
	}

	idx := indexOfAcceptanceCriterion(card.AcceptanceCriteria, criterionID)
	if idx < 0 {
		return model.AcceptanceCriterion{}, os.ErrNotExist
	}
	removed := card.AcceptanceCriteria[idx]
	card.AcceptanceCriteria = append(card.AcceptanceCriteria[:idx], card.AcceptanceCriteria[idx+1:]...)
	now := time.Now().UTC()
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.acceptance.deleted",
		Details:   fmt.Sprintf("criterion_id=%d", criterionID),
	})
	if err := s.writeCard(card); err != nil {
		return model.AcceptanceCriterion{}, err
	}
	return removed, nil
}

func (s *MarkdownStore) MoveCard(projectSlug string, number int, status string) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateStatus(status); err != nil {
		return model.Card{}, err
	}
	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Card{}, err
	}
	now := time.Now().UTC()
	card.Status = status
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{Timestamp: now, Type: "card.moved", Details: fmt.Sprintf("status=%s", status)})
	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) SetCardBranch(projectSlug string, number int, branch string) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	branch = strings.TrimSpace(branch)
	if err := validateBranchName(branch); err != nil {
		return model.Card{}, err
	}

	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Card{}, err
	}

	now := time.Now().UTC()
	card.Branch = branch
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{
		Timestamp: now,
		Type:      "card.branch.updated",
		Details:   fmt.Sprintf("branch=%s", branch),
	})
	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) DeleteCard(projectSlug string, number int, hard bool) (model.Card, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	card, err := s.getCardUnlocked(projectSlug, number)
	if err != nil {
		return model.Card{}, err
	}
	if hard {
		if err := os.Remove(s.cardPath(projectSlug, number)); err != nil {
			return model.Card{}, err
		}
		now := time.Now().UTC()
		card.UpdatedAt = now
		card.History = append(card.History, model.HistoryEvent{Timestamp: now, Type: "card.deleted_hard", Details: "file removed"})
		return card, nil
	}
	now := time.Now().UTC()
	card.Deleted = true
	card.UpdatedAt = now
	card.History = append(card.History, model.HistoryEvent{Timestamp: now, Type: "card.deleted_soft", Details: "marked deleted"})
	if err := s.writeCard(card); err != nil {
		return model.Card{}, err
	}
	return card, nil
}

func (s *MarkdownStore) Snapshot() ([]model.Project, []model.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	projects, err := s.ListProjects()
	if err != nil {
		return nil, nil, err
	}
	cards := make([]model.Card, 0)
	for _, p := range projects {
		projectCards, err := s.listProjectCards(p.Slug)
		if err != nil {
			return nil, nil, err
		}
		cards = append(cards, projectCards...)
	}
	return projects, cards, nil
}

func (s *MarkdownStore) listProjectCards(projectSlug string) ([]model.Card, error) {
	dirEntries, err := os.ReadDir(s.projectDir(projectSlug))
	if err != nil {
		return nil, err
	}
	cards := make([]model.Card, 0)
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "card-") || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		number, ok := cardNumberFromFilename(entry.Name())
		if !ok {
			continue
		}
		card, err := s.GetCard(projectSlug, number)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Number < cards[j].Number })
	return cards, nil
}

func (s *MarkdownStore) loadProject(slug string) (model.Project, error) {
	data, err := os.ReadFile(s.projectPath(slug))
	if err != nil {
		return model.Project{}, err
	}
	yml, _, err := splitFrontmatter(data)
	if err != nil {
		return model.Project{}, err
	}
	var fm projectFrontmatter
	if err := yaml.Unmarshal(yml, &fm); err != nil {
		return model.Project{}, err
	}
	if fm.Slug == "" {
		fm.Slug = slug
	}
	if fm.NextCardSeq <= 0 {
		fm.NextCardSeq = 1
	}
	return model.Project{
		Name:        fm.Name,
		Slug:        fm.Slug,
		LocalPath:   fm.LocalPath,
		RemoteURL:   fm.RemoteURL,
		CreatedAt:   fm.CreatedAt,
		UpdatedAt:   fm.UpdatedAt,
		NextCardSeq: fm.NextCardSeq,
	}, nil
}

func (s *MarkdownStore) writeProject(p model.Project) error {
	fm := projectFrontmatter{
		Name:        p.Name,
		Slug:        p.Slug,
		LocalPath:   p.LocalPath,
		RemoteURL:   p.RemoteURL,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		NextCardSeq: p.NextCardSeq,
	}
	yml, err := yaml.Marshal(&fm)
	if err != nil {
		return err
	}
	buf := bytes.Buffer{}
	buf.WriteString("---\n")
	buf.Write(yml)
	buf.WriteString("---\n")
	buf.WriteString("# Project\n")
	buf.WriteString(p.Name)
	buf.WriteByte('\n')
	return writeFileAtomic(s.projectPath(p.Slug), buf.Bytes(), 0o644)
}

func (s *MarkdownStore) writeCard(c model.Card) error {
	yml, body, err := serializeCard(c)
	if err != nil {
		return err
	}
	buf := bytes.Buffer{}
	buf.WriteString("---\n")
	buf.Write(yml)
	buf.WriteString("---\n")
	buf.WriteString(body)
	return writeFileAtomic(s.cardPath(c.ProjectSlug, c.Number), buf.Bytes(), 0o644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if err := renameFile(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}

func serializeCard(c model.Card) ([]byte, string, error) {
	fm := cardFrontmatter{
		ID:                        c.ID,
		ProjectSlug:               c.ProjectSlug,
		Number:                    c.Number,
		Title:                     c.Title,
		Branch:                    c.Branch,
		Status:                    c.Status,
		Deleted:                   c.Deleted,
		CreatedAt:                 c.CreatedAt,
		UpdatedAt:                 c.UpdatedAt,
		NextTodoID:                c.NextTodoID,
		NextAcceptanceCriterionID: c.NextAcceptanceCriterionID,
	}
	yml, err := yaml.Marshal(&fm)
	if err != nil {
		return nil, "", err
	}
	var body strings.Builder
	body.WriteString("# Description\n")
	writeTextEvents(&body, c.Description)
	body.WriteString("\n# Todos\n")
	writeTodos(&body, c.Todos)
	body.WriteString("\n# Acceptance Criteria\n")
	writeAcceptanceCriteria(&body, c.AcceptanceCriteria)
	body.WriteString("\n# Comments\n")
	writeTextEvents(&body, c.Comments)
	body.WriteString("\n# History\n")
	if len(c.History) == 0 {
		body.WriteString("(none)\n")
	} else {
		for _, event := range c.History {
			body.WriteString("## ")
			body.WriteString(event.Timestamp.UTC().Format(time.RFC3339))
			body.WriteString(" | ")
			body.WriteString(event.Type)
			body.WriteByte('\n')
			body.WriteString(strings.TrimSpace(event.Details))
			body.WriteString("\n\n")
		}
	}
	return yml, body.String(), nil
}

func writeTextEvents(body *strings.Builder, events []model.TextEvent) {
	if len(events) == 0 {
		body.WriteString("(none)\n")
		return
	}
	for _, event := range events {
		body.WriteString("## ")
		body.WriteString(event.Timestamp.UTC().Format(time.RFC3339))
		body.WriteByte('\n')
		body.WriteString(strings.TrimSpace(event.Body))
		body.WriteString("\n\n")
	}
}

func writeTodos(body *strings.Builder, todos []model.Todo) {
	if len(todos) == 0 {
		body.WriteString("(none)\n")
		return
	}
	for _, todo := range todos {
		status := "open"
		if todo.Completed {
			status = "done"
		}
		body.WriteString("## ")
		body.WriteString(strconv.Itoa(todo.ID))
		body.WriteString(" | ")
		body.WriteString(status)
		body.WriteByte('\n')
		body.WriteString(strings.TrimSpace(todo.Text))
		body.WriteString("\n\n")
	}
}

func writeAcceptanceCriteria(body *strings.Builder, criteria []model.AcceptanceCriterion) {
	if len(criteria) == 0 {
		body.WriteString("(none)\n")
		return
	}
	for _, criterion := range criteria {
		status := "open"
		if criterion.Completed {
			status = "done"
		}
		body.WriteString("## ")
		body.WriteString(strconv.Itoa(criterion.ID))
		body.WriteString(" | ")
		body.WriteString(status)
		body.WriteByte('\n')
		body.WriteString(strings.TrimSpace(criterion.Text))
		body.WriteString("\n\n")
	}
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

func indexOfTodo(todos []model.Todo, todoID int) int {
	for i := range todos {
		if todos[i].ID == todoID {
			return i
		}
	}
	return -1
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

func indexOfAcceptanceCriterion(criteria []model.AcceptanceCriterion, criterionID int) int {
	for i := range criteria {
		if criteria[i].ID == criterionID {
			return i
		}
	}
	return -1
}

func parseCard(data []byte) (model.Card, error) {
	yml, body, err := splitFrontmatter(data)
	if err != nil {
		return model.Card{}, err
	}
	var fm cardFrontmatter
	if err := yaml.Unmarshal(yml, &fm); err != nil {
		return model.Card{}, err
	}
	desc, todos, acceptanceCriteria, comments, history := parseSections(body)
	nextTodo := fm.NextTodoID
	if nextTodo <= 0 {
		nextTodo = nextTodoID(todos)
	}
	nextAcceptanceCriterion := fm.NextAcceptanceCriterionID
	if nextAcceptanceCriterion <= 0 {
		nextAcceptanceCriterion = nextAcceptanceCriterionID(acceptanceCriteria)
	}
	return model.Card{
		ID:                        fm.ID,
		ProjectSlug:               fm.ProjectSlug,
		Number:                    fm.Number,
		Title:                     fm.Title,
		Branch:                    fm.Branch,
		Status:                    fm.Status,
		Deleted:                   fm.Deleted,
		CreatedAt:                 fm.CreatedAt,
		UpdatedAt:                 fm.UpdatedAt,
		Description:               desc,
		Todos:                     todos,
		AcceptanceCriteria:        acceptanceCriteria,
		Comments:                  comments,
		History:                   history,
		NextTodoID:                nextTodo,
		NextAcceptanceCriterionID: nextAcceptanceCriterion,
	}, nil
}

func parseSections(body string) ([]model.TextEvent, []model.Todo, []model.AcceptanceCriterion, []model.TextEvent, []model.HistoryEvent) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		section string
		heading string
		lines   []string
		desc    []model.TextEvent
		todos   []model.Todo
		ac      []model.AcceptanceCriterion
		comm    []model.TextEvent
		hist    []model.HistoryEvent
	)

	flush := func() {
		if heading == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(lines, "\n"))
		if text == "" || text == "(none)" {
			heading = ""
			lines = nil
			return
		}
		switch section {
		case "Description":
			if ts, err := time.Parse(time.RFC3339, heading); err == nil {
				desc = append(desc, model.TextEvent{Timestamp: ts, Body: text})
			}
		case "Todos":
			parts := strings.SplitN(heading, " | ", 2)
			if len(parts) == 2 {
				todoID, idErr := strconv.Atoi(parts[0])
				if idErr == nil && todoID > 0 {
					status := strings.TrimSpace(parts[1])
					todos = append(todos, model.Todo{
						ID:        todoID,
						Text:      text,
						Completed: status == "done",
					})
				}
			}
		case "Acceptance Criteria":
			parts := strings.SplitN(heading, " | ", 2)
			if len(parts) == 2 {
				criterionID, idErr := strconv.Atoi(parts[0])
				if idErr == nil && criterionID > 0 {
					status := strings.TrimSpace(parts[1])
					ac = append(ac, model.AcceptanceCriterion{
						ID:        criterionID,
						Text:      text,
						Completed: status == "done",
					})
				}
			}
		case "Comments":
			if ts, err := time.Parse(time.RFC3339, heading); err == nil {
				comm = append(comm, model.TextEvent{Timestamp: ts, Body: text})
			}
		case "History":
			parts := strings.SplitN(heading, " | ", 2)
			if len(parts) == 2 {
				if ts, err := time.Parse(time.RFC3339, parts[0]); err == nil {
					hist = append(hist, model.HistoryEvent{Timestamp: ts, Type: parts[1], Details: text})
				}
			}
		}
		heading = ""
		lines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			flush()
			section = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			flush()
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if heading != "" {
			lines = append(lines, line)
		}
	}
	flush()
	return desc, todos, ac, comm, hist
}

func splitFrontmatter(data []byte) ([]byte, string, error) {
	raw := string(data)
	if !strings.HasPrefix(raw, "---\n") {
		return nil, "", errors.New("missing frontmatter")
	}
	rest := raw[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, "", errors.New("invalid frontmatter")
	}
	yml := rest[:idx]
	body := rest[idx+5:]
	return []byte(yml), body, nil
}

func validateStatus(status string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("status is required")
	}
	if _, ok := model.AllowedStatus[status]; !ok {
		return fmt.Errorf("invalid status %q", status)
	}
	return nil
}

func validateBranchName(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	if branch == "@" {
		return errors.New("invalid branch name")
	}
	if branch == "HEAD" {
		return errors.New("invalid branch name")
	}
	if strings.HasPrefix(branch, "refs/") {
		return errors.New("invalid branch name")
	}
	if strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return errors.New("invalid branch name")
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "@{") {
		return errors.New("invalid branch name")
	}
	if strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return errors.New("invalid branch name")
	}
	if strings.Contains(branch, "//") {
		return errors.New("invalid branch name")
	}
	for _, segment := range strings.Split(branch, "/") {
		if segment == "" {
			return errors.New("invalid branch name")
		}
		if strings.HasPrefix(segment, ".") {
			return errors.New("invalid branch name")
		}
		if strings.HasSuffix(segment, ".lock") {
			return errors.New("invalid branch name")
		}
	}
	for _, r := range branch {
		if r <= 0x20 || r == 0x7f {
			return errors.New("invalid branch name")
		}
		switch r {
		case '~', '^', ':', '?', '*', '[', '\\':
			return errors.New("invalid branch name")
		}
	}
	return nil
}

func Slugify(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	nonAlpha := regexp.MustCompile(`[^a-z0-9]+`)
	slug := nonAlpha.ReplaceAllString(base, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "project"
	}
	return slug
}

func cardNumberFromFilename(name string) (int, bool) {
	if !strings.HasPrefix(name, "card-") || !strings.HasSuffix(name, ".md") {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, "card-"), ".md")
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (s *MarkdownStore) projectDir(slug string) string {
	return filepath.Join(s.projectsDir, slug)
}

func (s *MarkdownStore) projectPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "project.md")
}

func (s *MarkdownStore) cardPath(projectSlug string, number int) string {
	return filepath.Join(s.projectDir(projectSlug), fmt.Sprintf("card-%d.md", number))
}
