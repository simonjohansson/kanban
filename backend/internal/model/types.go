package model

import "time"

var AllowedStatus = map[string]struct{}{
	"Todo":   {},
	"Doing":  {},
	"Review": {},
	"Done":   {},
}

type Project struct {
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	LocalPath   string    `json:"local_path,omitempty"`
	RemoteURL   string    `json:"remote_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	NextCardSeq int       `json:"next_card_seq"`
}

type TextEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Body      string    `json:"body"`
}

type HistoryEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Details   string    `json:"details"`
}

type Card struct {
	ID          string         `json:"id"`
	ProjectSlug string         `json:"project"`
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	Branch      string         `json:"branch"`
	Status      string         `json:"status"`
	Deleted     bool           `json:"deleted"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Description []TextEvent    `json:"description"`
	Comments    []TextEvent    `json:"comments"`
	History     []HistoryEvent `json:"history"`
}

type CardSummary struct {
	ID            string    `json:"id"`
	ProjectSlug   string    `json:"project"`
	Number        int       `json:"number"`
	Title         string    `json:"title"`
	Branch        string    `json:"branch"`
	Status        string    `json:"status"`
	Deleted       bool      `json:"deleted"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CommentsCount int       `json:"comments_count"`
	HistoryCount  int       `json:"history_count"`
}

type Event struct {
	Type      string    `json:"type"`
	Project   string    `json:"project"`
	CardID    string    `json:"card_id,omitempty"`
	CardNum   int       `json:"card_number,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
