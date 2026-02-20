package server

import (
	"testing"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/model"
	"github.com/stretchr/testify/require"
)

func TestHubPublishPreservesOrderWhenQueueHasCapacity(t *testing.T) {
	t.Parallel()

	h := &hub{broadcast: make(chan model.Event, 2)}
	first := model.Event{Type: "project.created", Project: "alpha", Timestamp: time.Now().UTC()}
	second := model.Event{Type: "card.created", Project: "alpha", Timestamp: time.Now().UTC()}

	h.Publish(first)
	h.Publish(second)

	require.Equal(t, first.Type, (<-h.broadcast).Type)
	require.Equal(t, second.Type, (<-h.broadcast).Type)
}

func TestHubPublishOverflowQueuesResyncFallback(t *testing.T) {
	t.Parallel()

	h := &hub{broadcast: make(chan model.Event, 1)}
	h.broadcast <- model.Event{Type: "card.created", Project: "alpha", Timestamp: time.Now().UTC()}

	h.Publish(model.Event{Type: "card.moved", Project: "alpha", Timestamp: time.Now().UTC()})

	event := <-h.broadcast
	require.Equal(t, "resync.required", event.Type)
	require.Equal(t, "alpha", event.Project)
}
