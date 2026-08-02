package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/api/response"
	"github.com/ai-desktop/assistant/internal/application"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

// Server HTTP 触发层
type Server struct {
	app    *application.AgentApp
	addr   string
	server *http.Server
}

func NewServer(app *application.AgentApp, addr string) *Server {
	return &Server{app: app, addr: addr}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/session/create", s.handleCreateSession)
	mux.HandleFunc("/api/v1/session/info", s.handleSessionInfo)
	mux.HandleFunc("/api/v1/session/messages", s.handleSessionMessages)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/chat/stream", s.handleChatStream)

	// CORS + 日志中间件
	handler := corsMiddleware(loggingMiddleware(mux))

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("[http] listening on %s\n", s.addr)
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
	resp := s.app.CreateSession(req)
	writeJSON(w, http.StatusOK, response.Success(resp))
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

func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response.ErrorFromCode(enums.InvalidParam))
		return
	}
	msgs, err := s.app.ListMessages(id, 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error(enums.SystemError.Code, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, response.Success(msgs))
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
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, response.Error(enums.InvalidParam.Code, "message required"))
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
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
			"type": "error", "content": err.Error(), "completed": true,
		}))
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
