package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// DeviceMessage 设备控制消息
type DeviceMessage struct {
	Type      string                 `json:"type"` // register|command|result|ping|event
	DeviceID  string                 `json:"deviceId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Content   string                 `json:"content,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// Hub WebSocket 设备中枢：手机 Agent 注册后可收发指令
type Hub struct {
	mu      sync.RWMutex
	devices map[string]*websocket.Conn // deviceId -> conn
	// 订阅会话事件的浏览器客户端
	clients map[*websocket.Conn]string // conn -> sessionId
}

func NewHub() *Hub {
	return &Hub{
		devices: make(map[string]*websocket.Conn),
		clients: make(map[*websocket.Conn]string),
	}
}

func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade: %v\n", err)
		return
	}
	deviceID := r.URL.Query().Get("deviceId")
	sessionID := r.URL.Query().Get("sessionId")
	role := r.URL.Query().Get("role") // device | console

	if role == "device" || deviceID != "" {
		if deviceID == "" {
			deviceID = "device-" + time.Now().Format("150405")
		}
		h.mu.Lock()
		h.devices[deviceID] = conn
		h.mu.Unlock()
		log.Printf("[ws] device registered: %s\n", deviceID)
		_ = conn.WriteJSON(DeviceMessage{Type: "event", Content: "registered", DeviceID: deviceID, Timestamp: time.Now().UnixMilli()})
		h.readDevice(deviceID, conn)
		return
	}

	// console client
	h.mu.Lock()
	h.clients[conn] = sessionID
	h.mu.Unlock()
	h.readConsole(conn)
}

func (h *Hub) readDevice(id string, conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.devices, id)
		h.mu.Unlock()
		_ = conn.Close()
	}()
	for {
		var msg DeviceMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		msg.DeviceID = id
		msg.Timestamp = time.Now().UnixMilli()
		// 广播给控制台
		h.broadcastConsole(msg)
	}
}

func (h *Hub) readConsole(conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		_ = conn.Close()
	}()
	for {
		var msg DeviceMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.Type == "command" && msg.DeviceID != "" {
			_ = h.SendToDevice(msg.DeviceID, msg)
		}
	}
}

func (h *Hub) SendToDevice(deviceID string, msg DeviceMessage) error {
	h.mu.RLock()
	conn := h.devices[deviceID]
	h.mu.RUnlock()
	if conn == nil {
		return errDeviceOffline
	}
	msg.Timestamp = time.Now().UnixMilli()
	return conn.WriteJSON(msg)
}

func (h *Hub) ListDevices() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.devices))
	for id := range h.devices {
		out = append(out, id)
	}
	return out
}

func (h *Hub) broadcastConsole(msg DeviceMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, _ := json.Marshal(msg)
	for c := range h.clients {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}

type offlineErr struct{}

func (offlineErr) Error() string { return "device offline" }

var errDeviceOffline = offlineErr{}
