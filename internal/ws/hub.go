package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/ricky40043/guess-who-game/internal/game"
)

type Envelope struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[string]*Client
	roomClients map[string]map[string]*Client
	service     *game.Service
}

func NewHub(service *game.Service) *Hub {
	return &Hub{
		clients:     make(map[string]*Client),
		roomClients: make(map[string]map[string]*Client),
		service:     service,
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	h.clients[client.ID] = client
	h.mu.Unlock()
}

func (h *Hub) AddToRoom(client *Client, roomID string) {
	h.mu.Lock()
	if h.roomClients[roomID] == nil {
		h.roomClients[roomID] = make(map[string]*Client)
	}
	h.roomClients[roomID][client.ID] = client
	client.RoomID = roomID
	h.mu.Unlock()
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	delete(h.clients, client.ID)
	if client.RoomID != "" {
		delete(h.roomClients[client.RoomID], client.ID)
		if len(h.roomClients[client.RoomID]) == 0 {
			delete(h.roomClients, client.RoomID)
		}
	}
	h.mu.Unlock()

	if client.RoomID == "" {
		return
	}
	if client.IsHost {
		h.service.SetHostConnected(client.RoomID, false)
		h.Broadcast(client.RoomID, "HOST_DISCONNECTED", map[string]any{"roomId": client.RoomID})
		return
	}
	if client.PlayerID != "" {
		if _, err := h.service.SetPlayerConnected(client.RoomID, client.PlayerID, false); err == nil {
			h.BroadcastPlayers(client.RoomID, "PLAYER_DISCONNECTED")
		}
	}
}

func marshalEnvelope(messageType string, data any) []byte {
	payload, err := json.Marshal(Envelope{Type: messageType, Data: data})
	if err != nil {
		log.Printf("marshal websocket message %s: %v", messageType, err)
		return nil
	}
	return payload
}

func (h *Hub) Broadcast(roomID, messageType string, data any) {
	payload := marshalEnvelope(messageType, data)
	if payload == nil {
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.roomClients[roomID]))
	for _, client := range h.roomClients[roomID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		client.Enqueue(payload)
	}
}

func (h *Hub) BroadcastPlayers(roomID, messageType string) {
	players, err := h.service.PlayerList(roomID)
	if err != nil {
		return
	}
	h.Broadcast(roomID, messageType, map[string]any{"players": players})
}

func (h *Hub) processEvent(roomID string, event *game.Event) {
	if event == nil {
		return
	}
	switch event.Type {
	case "QUESTION_STARTED":
		h.Broadcast(roomID, event.Type, event.Payload)
		go h.runAnswerTimer(roomID, event.Seq)
	case "GUESSING_STARTED":
		h.broadcastGuessing(roomID, event.Payload)
		go h.runGuessTimer(roomID, event.Seq)
	default:
		h.Broadcast(roomID, event.Type, event.Payload)
	}
}

func (h *Hub) broadcastGuessing(roomID string, common map[string]any) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.roomClients[roomID]))
	for _, client := range h.roomClients[roomID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		payload := common
		if !client.IsHost && client.PlayerID != "" {
			personalized, err := h.service.PersonalizedGuessInfo(roomID, client.PlayerID)
			if err == nil {
				payload = personalized
			}
		}
		client.Send("GUESSING_STARTED", payload)
	}
}

func (h *Hub) runAnswerTimer(roomID string, seq int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		remaining, event, stale := h.service.TickAnswer(roomID, seq)
		if stale {
			return
		}
		h.Broadcast(roomID, "TIMER_UPDATE", map[string]any{"phase": "answering", "timeLeft": remaining})
		if event != nil {
			h.processEvent(roomID, event)
			return
		}
	}
}

func (h *Hub) runGuessTimer(roomID string, seq int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		remaining, event, stale := h.service.TickGuess(roomID, seq)
		if stale {
			return
		}
		h.Broadcast(roomID, "TIMER_UPDATE", map[string]any{"phase": "guessing", "timeLeft": remaining})
		if event != nil {
			h.processEvent(roomID, event)
			return
		}
	}
}
