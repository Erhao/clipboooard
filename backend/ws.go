package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsMsg struct {
	Type string `json:"type"`
	Clip *Clip  `json:"clip"`
}

type Hub struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) Run() {
	select {}
}

func (h *Hub) Broadcast(clip *Clip) {
	data, err := json.Marshal(wsMsg{Type: "new_clip", Clip: clip})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Println("ws write:", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !checkToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}

	h.hub.mu.Lock()
	h.hub.clients[conn] = true
	h.hub.mu.Unlock()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.hub.mu.Lock()
			delete(h.hub.clients, conn)
			h.hub.mu.Unlock()
			conn.Close()
			return
		}
	}
}
