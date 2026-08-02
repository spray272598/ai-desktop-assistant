package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/api/response"
	"github.com/ai-desktop/assistant/internal/application"
	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

// Server HTTP 触发层
type Server struct {
	app          *application.AgentApp
	toolDescFunc func() []map[string]string
	webDir       string
	addr         string
	server       *http.Server
}

func NewServer(app *application.AgentApp, addr string) *Server {
	return &Server{app: app, addr: addr, webDir: "web"}
}

func (s *Server) WithToolDescriptions(fn func() []map[string]string) *Server {
	s.toolDescFunc = fn
	return s
}

func (s *Server) WithWebDir(dir string) *Server {
	s.webDir = dir
	return s
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)

	// session
	mux.HandleFunc("/api/v1/session/create", s.handleCreateSession)
	mux.HandleFunc("/api/v1/session/info", s.handleSessionInfo)
	mux.HandleFunc("/api/v1/session/list", s.handleSessionList)
	mux.HandleFunc("/api/v1/session/messages", s.handleSessionMessages)

	// chat / tools
	mux.HandleFunc("/api/v1/tools", s.handleTools)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/chat/stream", s.handleChatStream)

	// permission
	mux.HandleFunc("/api/v1/permission/pending", s.handlePermissionPending)
	mux.HandleFunc("/api/v1/permission/approve", s.handlePermissionApprove)
	mux.HandleFunc("/api/v1/permission/reject", s.handlePermissionReject)

	// MCP 热加载
	mux.HandleFunc("/api/v1/mcp/servers", s.handleMCPServers)
	mux.HandleFunc("/api/v1/mcp/servers/delete", s.handleMCPDelete)
	mux.HandleFunc("/api/v1/mcp/reload", s.handleMCPReload)

	// Web 静态壳
	mux.HandleFunc("/", s.handleWeb)

	handler := corsMiddleware(loggingMiddleware(mux))
	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("[http] listening on %s (web=%s)\n", s.addr, s.webDir)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response.Success(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}))
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	var req dto.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(s.app.CreateSession(req)))
}

func (s *Server) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	info := s.app.GetSessionInfo(id)
	if info == nil {
		writeJSON(w, http.StatusNotFound, response.ErrorFromCode(enums.SessionNotFound))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(info))
}

func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = "anonymous"
	}
	list, err := s.app.ListSessionsByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(list))
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	msgs, err := s.app.ListMessages(id, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(msgs))
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if s.toolDescFunc == nil {
		writeJSON(w, http.StatusOK, response.Success([]map[string]string{}))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(s.toolDescFunc()))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	var req dto.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "message required"))
		return
	}
	resp, err := s.app.Chat(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(resp))
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	var req dto.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, "streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, err := s.app.ChatStream(req)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{"type": "error", "content": err.Error(), "completed": true}))
		flusher.Flush()
		return
	}
	for ev := range ch {
		b, _ := json.Marshal(ev)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
		if ev.Completed {
			break
		}
	}
}

// ---- permission ----

func (s *Server) handlePermissionPending(w http.ResponseWriter, r *http.Request) {
	g := s.app.PermissionGuard()
	if g == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	sid := r.URL.Query().Get("sessionId")
	writeJSON(w, http.StatusOK, response.Success(g.ListPending(sid)))
}

func (s *Server) handlePermissionApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	g := s.app.PermissionGuard()
	if g == nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, "permission guard unavailable"))
		return
	}
	var req dto.PermissionApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "id required"))
		return
	}
	if req.Scope == "" {
		req.Scope = "once"
	}
	p, err := g.Approve(req.ID, req.Scope)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]interface{}{
		"approved": true, "pending": p, "hint": "请重新发送指令或发送「继续」以执行已批准操作",
	}))
}

func (s *Server) handlePermissionReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	g := s.app.PermissionGuard()
	if g == nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, "permission guard unavailable"))
		return
	}
	var req dto.PermissionApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "id required"))
		return
	}
	if err := g.Reject(req.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]bool{"rejected": true}))
}

// ---- MCP ----

func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := svc.ListServers(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, response.Success(list))
	case http.MethodPost:
		var req dto.MCPServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		cfg := entity.ServerConfig{
			Name: req.Name, Transport: req.Transport, Command: req.Command,
			Args: req.Args, Env: req.Env, URL: req.URL, Enabled: enabled, TimeoutSec: req.TimeoutSec,
		}
		if err := svc.UpsertServer(r.Context(), cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, response.Success(map[string]interface{}{"ok": true, "name": cfg.Name}))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
	}
}

func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "mcp disabled"))
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		name = body.Name
	}
	if name == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "name required"))
		return
	}
	if err := svc.DeleteServer(r.Context(), name); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]bool{"deleted": true}))
}

func (s *Server) handleMCPReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "mcp disabled"))
		return
	}
	if err := svc.ReloadFromDB(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]bool{"reloaded": true}))
}

// ---- web ----

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	// API 路径不应落到这里
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
		http.NotFound(w, r)
		return
	}
	dir := s.webDir
	if dir == "" {
		dir = "web"
	}
	// 默认 index
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}
	full := filepath.Join(dir, filepath.Clean("/"+path))
	// 防止越界
	absDir, _ := filepath.Abs(dir)
	absFile, _ := filepath.Abs(full)
	if !strings.HasPrefix(absFile, absDir) {
		http.NotFound(w, r)
		return
	}
	if st, err := os.Stat(absFile); err != nil || st.IsDir() {
		// SPA fallback
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return
		}
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, absFile)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[http] %s %s %s\n", r.Method, r.URL.Path, time.Since(start))
	})
}

// 保证 fs 被引用（避免某些 go 版本 unused）
var _ fs.FS
