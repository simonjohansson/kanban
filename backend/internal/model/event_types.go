package model

type EventType string

const (
	EventTypeProjectCreated        EventType = "project.created"
	EventTypeProjectDeleted        EventType = "project.deleted"
	EventTypeCardCreated           EventType = "card.created"
	EventTypeCardBranchUpdated     EventType = "card.branch.updated"
	EventTypeCardMoved             EventType = "card.moved"
	EventTypeCardCommented         EventType = "card.commented"
	EventTypeCardUpdated           EventType = "card.updated"
	EventTypeCardTodoAdded         EventType = "card.todo.added"
	EventTypeCardTodoUpdated       EventType = "card.todo.updated"
	EventTypeCardTodoDeleted       EventType = "card.todo.deleted"
	EventTypeCardAcceptanceAdded   EventType = "card.acceptance.added"
	EventTypeCardAcceptanceUpdated EventType = "card.acceptance.updated"
	EventTypeCardAcceptanceDeleted EventType = "card.acceptance.deleted"
	EventTypeCardDeletedSoft       EventType = "card.deleted_soft"
	EventTypeCardDeletedHard       EventType = "card.deleted_hard"
	EventTypeResyncRequired        EventType = "resync.required"
)

var websocketEventTypes = []EventType{
	EventTypeProjectCreated,
	EventTypeProjectDeleted,
	EventTypeCardCreated,
	EventTypeCardBranchUpdated,
	EventTypeCardMoved,
	EventTypeCardCommented,
	EventTypeCardUpdated,
	EventTypeCardTodoAdded,
	EventTypeCardTodoUpdated,
	EventTypeCardTodoDeleted,
	EventTypeCardAcceptanceAdded,
	EventTypeCardAcceptanceUpdated,
	EventTypeCardAcceptanceDeleted,
	EventTypeCardDeletedSoft,
	EventTypeCardDeletedHard,
	EventTypeResyncRequired,
}

func WebSocketEventTypes() []EventType {
	out := make([]EventType, len(websocketEventTypes))
	copy(out, websocketEventTypes)
	return out
}
