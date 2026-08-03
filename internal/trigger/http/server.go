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
	wsHandler    http.HandlerFunc
	modelRoutes  func() interface{}
	listDevices  func() []string
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

func (s *Server) WithWebSocket(h http.HandlerFunc) *Server {
	s.wsHandler = h
	return s
}

func (s *Server) WithDeviceList(fn func() []string) *Server {
	s.listDevices = fn
	return s
}

func (s *Server) WithModelRoutes(fn func() interface{}) *Server {
	s.modelRoutes = fn
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
	mux.HandleFunc("/api/v1/session/export", s.handleSessionExport)
	mux.HandleFunc("/api/v1/session/import", s.handleSessionImport)

	// chat / tools
	mux.HandleFunc("/api/v1/tools", s.handleTools)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/chat/stream", s.handleChatStream)

	// permission
	mux.HandleFunc("/api/v1/permission/pending", s.handlePermissionPending)
	mux.HandleFunc("/api/v1/permission/approve", s.handlePermissionApprove)
	mux.HandleFunc("/api/v1/permission/reject", s.handlePermissionReject)

	// MCP 热加载 + 插件市场 + 健康/工具
	mux.HandleFunc("/api/v1/mcp/servers", s.handleMCPServers)
	mux.HandleFunc("/api/v1/mcp/servers/delete", s.handleMCPDelete)
	mux.HandleFunc("/api/v1/mcp/install", s.handleMCPInstallCustom)
	mux.HandleFunc("/api/v1/mcp/reload", s.handleMCPReload)
	mux.HandleFunc("/api/v1/mcp/health", s.handleMCPHealth)
	mux.HandleFunc("/api/v1/mcp/tools", s.handleMCPTools)
	mux.HandleFunc("/api/v1/mcp/market", s.handleMCPMarket)
	mux.HandleFunc("/api/v1/mcp/market/install", s.handleMCPMarketInstall)
	mux.HandleFunc("/api/v1/mcp/market/uninstall", s.handleMCPMarketUninstall)

	// Skills
	mux.HandleFunc("/api/v1/skills", s.handleSkills)
	mux.HandleFunc("/api/v1/skills/install", s.handleSkillInstall)
	mux.HandleFunc("/api/v1/skills/uninstall", s.handleSkillUninstall)
	mux.HandleFunc("/api/v1/skills/reload", s.handleSkillReload)

	// 模型 A/B
	mux.HandleFunc("/api/v1/models", s.handleModels)

	// WebSocket 设备控制
	if s.wsHandler != nil {
		mux.HandleFunc("/ws", s.wsHandler)
		mux.HandleFunc("/api/v1/devices", s.handleDevices)
	}

	// Web（优先 dist，再 web）
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
	out := map[string]interface{}{
		"approved": true, "pending": p,
		"hint": "已批准。发送「继续」或在 approve 时设 continue=true 自动恢复执行",
	}
	// 批准后自动继续 Agent
	if req.Continue {
		sid := req.SessionID
		if sid == "" && p != nil {
			sid = p.SessionID
		}
		uid := req.UserID
		if uid == "" {
			uid = "anonymous"
		}
		if sid != "" {
			chatRes, err := s.app.ContinueAfterPermission(sid, uid)
			if err != nil {
				out["continueError"] = err.Error()
			} else {
				out["continued"] = true
				out["chat"] = chatRes
			}
		}
	}
	writeJSON(w, http.StatusOK, response.Success(out))
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
		res, err := svc.InstallCustom(r.Context(), cfg)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, response.Success(res))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
	}
}

// handleMCPInstallCustom POST 自定义安装 npx/binary/SSE
func (s *Server) handleMCPInstallCustom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "mcp disabled"))
		return
	}
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
	res, err := svc.InstallCustom(r.Context(), cfg)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(res))
}

func (s *Server) handleMCPHealth(w http.ResponseWriter, r *http.Request) {
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(svc.Health(r.Context())))
}

func (s *Server) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	svc := s.app.MCPService()
	if svc == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	server := r.URL.Query().Get("server")
	list, err := svc.ListToolsByServer(r.Context(), server)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, t := range list {
		out = append(out, map[string]interface{}{
			"name": t.Name, "description": t.Description, "server": t.ServerName,
			"inputSchema": t.InputSchema,
		})
	}
	writeJSON(w, http.StatusOK, response.Success(out))
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

func (s *Server) handleMCPMarket(w http.ResponseWriter, r *http.Request) {
	m := s.app.Marketplace()
	if m == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(m.List(r.Context())))
}

func (s *Server) handleMCPMarketInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	m := s.app.Marketplace()
	if m == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "marketplace unavailable"))
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "id required"))
		return
	}
	res, err := m.Install(r.Context(), body.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(res))
}

func (s *Server) handleMCPMarketUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	m := s.app.Marketplace()
	if m == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "marketplace unavailable"))
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "id required"))
		return
	}
	if err := m.Uninstall(r.Context(), body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]bool{"uninstalled": true}))
}

// ---- Skills ----

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	sk := s.app.Skills()
	if sk == nil {
		writeJSON(w, http.StatusOK, response.Success([]interface{}{}))
		return
	}
	if id := r.URL.Query().Get("id"); id != "" {
		one := sk.Get(id)
		if one == nil {
			writeJSON(w, http.StatusNotFound, response.Error(enums.InvalidParam.Code, "skill not found"))
			return
		}
		writeJSON(w, http.StatusOK, response.Success(one))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(sk.List()))
}

func (s *Server) handleSkillInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	sk := s.app.Skills()
	if sk == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "skills disabled"))
		return
	}
	var body struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "path required (skill dir or SKILL.md)"))
		return
	}
	installed, err := sk.InstallFromPath(body.Path, body.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(installed))
}

func (s *Server) handleSkillUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	sk := s.app.Skills()
	if sk == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "skills disabled"))
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "id required"))
		return
	}
	if err := sk.Uninstall(body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]bool{"uninstalled": true}))
}

func (s *Server) handleSkillReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	sk := s.app.Skills()
	if sk == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "skills disabled"))
		return
	}
	if err := sk.Reload(); err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]interface{}{
		"reloaded": true, "count": len(sk.List()),
	}))
}

func (s *Server) handleSessionExport(w http.ResponseWriter, r *http.Request) {
	exp := s.app.Export()
	if exp == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "export unavailable"))
		return
	}
	sid := r.URL.Query().Get("sessionId")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "sessionId required"))
		return
	}
	var path string
	var err error
	if format == "md" || format == "markdown" {
		path, err = exp.ExportMarkdown(r.Context(), sid)
	} else {
		path, err = exp.ExportJSON(r.Context(), sid)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]string{"path": path, "format": format}))
}

func (s *Server) handleSessionImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	exp := s.app.Export()
	if exp == nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "export unavailable"))
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "path required"))
		return
	}
	id, err := exp.ImportJSON(r.Context(), body.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]string{"sessionId": id}))
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if s.modelRoutes == nil {
		writeJSON(w, http.StatusOK, response.Success(map[string]interface{}{}))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(s.modelRoutes()))
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := []string{}
	if s.listDevices != nil {
		devices = s.listDevices()
	}
	writeJSON(w, http.StatusOK, response.Success(map[string]interface{}{
		"devices": devices,
		"ws":      "/ws?role=device&deviceId=phone-1",
		"hint":    "手机 Agent 连接 WebSocket 后可收发 command/result",
	}))
}

// ---- web ----

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" || r.URL.Path == "/ws" {
		http.NotFound(w, r)
		return
	}
	// 优先 React 构建产物 web/dist，其次 web/
	dirs := []string{}
	if s.webDir != "" {
		dirs = append(dirs, filepath.Join(s.webDir, "dist"), s.webDir)
	}
	dirs = append(dirs, "web/dist", "web")
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}
	for _, dir := range dirs {
		full := filepath.Join(dir, filepath.Clean("/"+path))
		absDir, _ := filepath.Abs(dir)
		absFile, _ := filepath.Abs(full)
		if !strings.HasPrefix(strings.ToLower(absFile), strings.ToLower(absDir)) {
			continue
		}
		if st, err := os.Stat(absFile); err == nil && !st.IsDir() {
			http.ServeFile(w, r, absFile)
			return
		}
		// SPA fallback
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err == nil && (path == "/index.html" || !strings.Contains(path, ".")) {
			http.ServeFile(w, r, index)
			return
		}
	}
	http.NotFound(w, r)
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
