package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ricky40043/guess-who-game/internal/game"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second
	maxMessageSize = 32 * 1024
)

type Client struct {
	ID       string
	RoomID   string
	PlayerID string
	IsHost   bool

	conn      *rawConn
	hub       *Hub
	send      chan []byte
	closeOnce sync.Once
}

func NewClient(conn *rawConn, hub *Hub) *Client {
	return &Client{ID: randomClientID(), conn: conn, hub: hub, send: make(chan []byte, 64)}
}

func randomClientID() string {
	return time.Now().Format("20060102150405.000000000") + "-" + strings.ReplaceAll(time.Now().String(), " ", "")
}

func (c *Client) Enqueue(payload []byte) {
	defer func() { _ = recover() }()
	select {
	case c.send <- payload:
	default:
		log.Printf("websocket send queue full: %s", c.ID)
	}
}

func (c *Client) Send(messageType string, data any) {
	payload := marshalEnvelope(messageType, data)
	if payload != nil {
		c.Enqueue(payload)
	}
}

func (c *Client) Error(code, message string) {
	c.Send("ERROR", map[string]any{"code": code, "message": message})
}

func decode(data any, target any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var message Envelope
		if err := json.Unmarshal(raw, &message); err != nil {
			c.Error("BAD_MESSAGE", "訊息格式錯誤")
			continue
		}
		c.handle(&message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) requireHost() bool {
	if !c.IsHost || c.RoomID == "" {
		c.Error("NOT_HOST", "只有房主可以執行")
		return false
	}
	return true
}

func (c *Client) handle(message *Envelope) {
	switch message.Type {
	case "CREATE_ROOM":
		var request struct {
			Settings game.Settings `json:"settings"`
		}
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "建房參數錯誤")
			return
		}
		room := c.hub.service.CreateRoom(c.ID, request.Settings)
		c.IsHost = true
		c.PlayerID = c.ID
		c.hub.AddToRoom(c, room.ID)
		c.Send("ROOM_CREATED", map[string]any{
			"roomId": room.ID, "hostToken": room.HostToken, "status": room.Status,
			"settings": room.Settings, "players": room.PlayerList(),
		})

	case "JOIN_ROOM":
		var request struct {
			RoomID string `json:"roomId"`
			Name   string `json:"name"`
		}
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "加入房間參數錯誤")
			return
		}
		player, err := c.hub.service.JoinRoom(request.RoomID, c.ID, request.Name)
		if err != nil {
			c.Error("JOIN_FAILED", err.Error())
			return
		}
		roomID := strings.ToUpper(strings.TrimSpace(request.RoomID))
		c.PlayerID = player.ID
		c.IsHost = false
		c.hub.AddToRoom(c, roomID)
		snapshot, _ := c.hub.service.Snapshot(roomID, player.ID, false)
		c.Send("JOIN_SUCCESS", snapshot)
		c.hub.BroadcastPlayers(roomID, "PLAYER_JOINED")

	case "REJOIN_ROOM":
		var request struct {
			RoomID    string `json:"roomId"`
			PlayerID  string `json:"playerId"`
			HostToken string `json:"hostToken"`
		}
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "重連參數錯誤")
			return
		}
		roomID := strings.ToUpper(strings.TrimSpace(request.RoomID))
		if request.HostToken != "" {
			room, err := c.hub.service.ReconnectHost(roomID, request.HostToken, c.ID)
			if err != nil {
				c.Error("REJOIN_FAILED", err.Error())
				return
			}
			c.IsHost = true
			c.PlayerID = c.ID
			c.hub.AddToRoom(c, room.ID)
			snapshot, _ := c.hub.service.Snapshot(room.ID, "", true)
			snapshot["hostToken"] = room.HostToken
			c.Send("REJOIN_SUCCESS", snapshot)
			c.hub.Broadcast(room.ID, "HOST_RECONNECTED", map[string]any{"roomId": room.ID})
			return
		}
		if _, err := c.hub.service.SetPlayerConnected(roomID, request.PlayerID, true); err != nil {
			c.Error("REJOIN_FAILED", err.Error())
			return
		}
		c.IsHost = false
		c.PlayerID = request.PlayerID
		c.hub.AddToRoom(c, roomID)
		snapshot, err := c.hub.service.Snapshot(roomID, request.PlayerID, false)
		if err != nil {
			c.Error("REJOIN_FAILED", err.Error())
			return
		}
		c.Send("REJOIN_SUCCESS", snapshot)
		c.hub.BroadcastPlayers(roomID, "PLAYER_REJOINED")

	case "UPDATE_SETTINGS":
		if !c.requireHost() {
			return
		}
		var request game.Settings
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "設定格式錯誤")
			return
		}
		settings, err := c.hub.service.UpdateSettings(c.RoomID, request)
		if err != nil {
			c.Error("UPDATE_FAILED", err.Error())
			return
		}
		c.hub.Broadcast(c.RoomID, "SETTINGS_UPDATED", map[string]any{"settings": settings})

	case "START_GAME":
		if !c.requireHost() {
			return
		}
		event, err := c.hub.service.StartGame(c.RoomID)
		if err != nil {
			c.Error("START_FAILED", err.Error())
			return
		}
		c.hub.Broadcast(c.RoomID, "GAME_STARTED", map[string]any{"roomId": c.RoomID})
		c.hub.processEvent(c.RoomID, event)

	case "SUBMIT_ANSWER":
		if c.IsHost || c.PlayerID == "" {
			c.Error("NOT_PLAYER", "房主控制畫面不能作答，請用手機加入")
			return
		}
		var request struct {
			QuestionIndex int    `json:"questionIndex"`
			Answer        string `json:"answer"`
		}
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "答案格式錯誤")
			return
		}
		progress, event, err := c.hub.service.SubmitAnswer(c.RoomID, c.PlayerID, request.QuestionIndex, request.Answer)
		if err != nil {
			c.Error("ANSWER_FAILED", err.Error())
			return
		}
		c.Send("ANSWER_ACCEPTED", map[string]any{"questionIndex": request.QuestionIndex, "answer": request.Answer})
		c.hub.Broadcast(c.RoomID, "ANSWER_PROGRESS", progress)
		c.hub.processEvent(c.RoomID, event)

	case "NEXT_REVEAL":
		if !c.requireHost() {
			return
		}
		payload, done, err := c.hub.service.NextReveal(c.RoomID)
		if err != nil {
			c.Error("REVEAL_FAILED", err.Error())
			return
		}
		if done {
			c.hub.Broadcast(c.RoomID, "REVEAL_COMPLETE", payload)
			return
		}
		c.hub.Broadcast(c.RoomID, "PROFILE_REVEALED", payload)

	case "START_GUESSING":
		if !c.requireHost() {
			return
		}
		event, err := c.hub.service.StartGuessing(c.RoomID)
		if err != nil {
			c.Error("GUESS_START_FAILED", err.Error())
			return
		}
		c.hub.processEvent(c.RoomID, event)

	case "SUBMIT_GUESSES":
		if c.IsHost || c.PlayerID == "" {
			c.Error("NOT_PLAYER", "房主控制畫面不能提交猜測")
			return
		}
		var request struct {
			Guesses map[string]string `json:"guesses"`
		}
		if err := decode(message.Data, &request); err != nil {
			c.Error("BAD_PAYLOAD", "配對格式錯誤")
			return
		}
		progress, event, err := c.hub.service.SubmitGuesses(c.RoomID, c.PlayerID, request.Guesses)
		if err != nil {
			c.Error("GUESS_FAILED", err.Error())
			return
		}
		c.Send("GUESS_ACCEPTED", map[string]any{"submitted": true})
		c.hub.Broadcast(c.RoomID, "GUESS_PROGRESS", progress)
		c.hub.processEvent(c.RoomID, event)

	case "LEAVE_ROOM":
		if c.RoomID == "" {
			return
		}
		if c.IsHost {
			c.hub.Broadcast(c.RoomID, "ROOM_CLOSED", map[string]any{"message": "房主已關閉房間"})
			c.hub.service.DeleteRoom(c.RoomID)
		}
		c.RoomID = ""

	case "PING":
		c.Send("PONG", map[string]any{"time": time.Now().UnixMilli()})
	default:
		c.Error("UNKNOWN_TYPE", "不支援的訊息類型")
	}
}

func Serve(hub *Hub, writer http.ResponseWriter, request *http.Request) {
	conn, err := upgrade(writer, request)
	if err != nil {
		log.Printf("upgrade websocket: %v", err)
		return
	}
	client := NewClient(conn, hub)
	hub.Register(client)
	go client.writePump()
	go client.readPump()
}
