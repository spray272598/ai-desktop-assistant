package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/bootstrap"
	"github.com/ai-desktop/assistant/internal/infrastructure/config"
	httpserver "github.com/ai-desktop/assistant/internal/trigger/http"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	mode := flag.String("mode", "http", "run mode: http | repl")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := bootstrap.Build(cfg)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	fmt.Println("🚀 AI Desktop Assistant")
	fmt.Printf("   version=%s mode=%s\n", cfg.Agent.Version, *mode)
	fmt.Println("📋 已注册工具:")
	for _, t := range app.Tools.ListTools() {
		fmt.Printf("   - %s: %s\n", t.Name(), t.Description())
	}

	switch strings.ToLower(*mode) {
	case "repl":
		runREPL(app)
	default:
		runHTTP(app, cfg)
	}
}

func runHTTP(app *bootstrap.App, cfg *config.Config) {
	srv := httpserver.NewServer(app.AgentApp, cfg.Addr())

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("http server stopped: %v\n", err)
		}
	}()

	fmt.Printf("🌐 HTTP 服务: http://%s\n", cfg.Addr())
	fmt.Println("   POST /api/v1/session/create")
	fmt.Println("   POST /api/v1/chat")
	fmt.Println("   POST /api/v1/chat/stream  (SSE)")
	fmt.Println("   GET  /health")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\n👋 shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func runREPL(app *bootstrap.App) {
	fmt.Println("💬 REPL 模式 (输入 quit 退出)")
	reader := bufio.NewReader(os.Stdin)

	session := app.AgentApp.CreateSession(dto.CreateSessionRequest{
		AgentID: "desktop-agent",
		UserID:  "local-user",
		Title:   "REPL 会话",
	})
	fmt.Printf("📁 sessionId=%s\n\n", session.SessionID)

	for {
		fmt.Print("你: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" {
			fmt.Println("👋 再见")
			return
		}

		fmt.Print("\n🤖 AI: ")
		resp, err := app.AgentApp.Chat(dto.ChatRequest{
			SessionID: session.SessionID,
			Message:   input,
			AgentID:   "desktop-agent",
			UserID:    "local-user",
		})
		if err != nil {
			fmt.Printf("❌ %v\n\n", err)
			continue
		}
		fmt.Println(resp.Response)
		fmt.Printf("\n📊 intent=%s steps=%d tools=%d tokens≈%d\n\n",
			resp.Intent, resp.Steps, resp.ToolCalls, resp.TokenUsed)
	}
}
