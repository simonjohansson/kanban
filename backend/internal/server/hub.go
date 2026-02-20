package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/simonjohansson/kanban/backend/internal/model"
)

type wsClient struct {
	conn    *websocket.Conn
	project string
	mu      sync.Mutex
}

type hub struct {
	upgrader   websocket.Upgrader
	register   chan *wsClient
	unregister chan *wsClient
	broadcast  chan model.Event
	done       chan struct{}
	clients    map[*wsClient]struct{}
}

func newHub() *hub {
	h := &hub{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		broadcast:  make(chan model.Event, 128),
		done:       make(chan struct{}),
		clients:    make(map[*wsClient]struct{}),
	}
	go h.run()
	return h
}

func (h *hub) Close() {
	close(h.done)
}

func (h *hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{conn: conn, project: r.URL.Query().Get("project")}
	h.register <- client

	go func() {
		defer func() { h.unregister <- client }()
		for {
			if _, _, err := client.conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *hub) Publish(event model.Event) {
	select {
	case h.broadcast <- event:
	default:
		// Drop when saturated to avoid blocking request flow.
	}
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				_ = client.conn.Close()
			}
		case event := <-h.broadcast:
			for client := range h.clients {
				if client.project != "" && client.project != event.Project {
					continue
				}
				client.mu.Lock()
				_ = client.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err := client.conn.WriteJSON(event)
				client.mu.Unlock()
				if err != nil {
					delete(h.clients, client)
					_ = client.conn.Close()
				}
			}
		case <-h.done:
			for client := range h.clients {
				_ = client.conn.Close()
			}
			return
		}
	}
}
