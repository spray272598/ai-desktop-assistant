package main

// Wails 桌面窗口入口（P2）
//
// 使用方式（需安装 Wails CLI）:
//   go install github.com/wailsapp/wails/v2/cmd/wails@latest
//   cd cmd/desktop && wails dev
//
// 当前提供可编译的占位：启动时提示用内嵌 WebView 加载本地控制台。
// 完整 Wails 项目可执行 `wails init` 后把 frontend 指向 ../../web 。

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	url := "http://127.0.0.1:8080"
	if v := os.Getenv("ASSISTANT_URL"); v != "" {
		url = v
	}
	fmt.Println("AI Desktop Assistant — Desktop Shell (Wails-ready)")
	fmt.Println("1) 请先启动后端: go run ./cmd/server")
	fmt.Println("2) 本程序将尝试打开系统浏览器到控制台:", url)
	fmt.Println()
	fmt.Println("正式嵌入窗口请安装 Wails 后初始化:")
	fmt.Println("  wails init -n ai-desktop-ui -t react-ts")
	fmt.Println("  将 frontend 指向仓库 web/ 目录并 proxy 到 :8080")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
