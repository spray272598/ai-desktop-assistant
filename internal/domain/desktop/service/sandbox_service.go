package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SandboxService 代码沙箱：优先 Docker，失败则受限本地临时目录执行
type SandboxService struct {
	WorkRoot   string
	UseDocker  bool
	DockerImage string
	MaxTimeout time.Duration
}

func NewSandboxService(workRoot string, useDocker bool) *SandboxService {
	if workRoot == "" {
		workRoot = "./temp/sandbox"
	}
	_ = os.MkdirAll(workRoot, 0755)
	return &SandboxService{
		WorkRoot:    workRoot,
		UseDocker:   useDocker,
		DockerImage: "python:3.11-alpine",
		MaxTimeout:  30 * time.Second,
	}
}

// Run 执行代码 language=python|javascript|js
func (s *SandboxService) Run(ctx context.Context, language, code string) (string, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("code required")
	}
	switch language {
	case "python", "py":
		language = "python"
	case "javascript", "js", "node":
		language = "javascript"
	default:
		return "", fmt.Errorf("unsupported language: %s (python|javascript)", language)
	}

	if s.UseDocker && dockerAvailable() {
		out, err := s.runDocker(ctx, language, code)
		if err == nil {
			return out, nil
		}
		// 降级本地
	}
	return s.runLocal(ctx, language, code)
}

func (s *SandboxService) runDocker(ctx context.Context, language, code string) (string, error) {
	dir, err := os.MkdirTemp(s.WorkRoot, "sbx-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	var filename, image, cmdName string
	switch language {
	case "python":
		filename, image, cmdName = "main.py", s.DockerImage, "python"
	default:
		filename, image, cmdName = "main.js", "node:20-alpine", "node"
	}
	scriptPath := filepath.Join(dir, filename)
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return "", err
	}

	timeout := s.MaxTimeout
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 && d < timeout {
			timeout = d
		}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// docker run --rm -v dir:/work -w /work --network none --memory 128m image cmd file
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--memory", "128m",
		"--cpus", "0.5",
		"-v", dir + ":/work:ro",
		"-w", "/work",
		image, cmdName, filename,
	}
	cmd := exec.CommandContext(cctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if err != nil {
		return fmt.Sprintf("exit_error: %v\nstdout:\n%s\nstderr:\n%s", err, out, errOut), nil
	}
	if errOut != "" {
		return out + "\n[stderr]\n" + errOut, nil
	}
	return out, nil
}

func (s *SandboxService) runLocal(ctx context.Context, language, code string) (string, error) {
	dir, err := os.MkdirTemp(s.WorkRoot, "sbx-local-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	var filename, bin string
	var args []string
	switch language {
	case "python":
		filename = "main.py"
		bin = "python"
		if runtime.GOOS == "windows" {
			if _, e := exec.LookPath("python"); e != nil {
				bin = "py"
			}
		}
		args = []string{filepath.Join(dir, filename)}
	default:
		filename = "main.js"
		bin = "node"
		args = []string{filepath.Join(dir, filename)}
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(code), 0644); err != nil {
		return "", err
	}
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("本地未找到 %s，且 Docker 不可用: %w", bin, err)
	}

	cctx, cancel := context.WithTimeout(ctx, s.MaxTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, args...)
	cmd.Dir = dir
	// 清空部分环境，降低风险
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TEMP=" + dir, "TMP=" + dir}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if err != nil {
		return fmt.Sprintf("[local-sandbox] error: %v\nstdout:\n%s\nstderr:\n%s", err, out, errOut), nil
	}
	if errOut != "" {
		return out + "\n[stderr]\n" + errOut, nil
	}
	return "[local-sandbox]\n" + out, nil
}

func dockerAvailable() bool {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}
