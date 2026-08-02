package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserService 浏览器自动化（chromedp，失败时 HTTP 降级）
type BrowserService struct {
	mu       sync.Mutex
	saveDir  string
	headless bool
	timeout  time.Duration
}

func NewBrowserService(saveDir string) *BrowserService {
	if saveDir == "" {
		saveDir = "./screenshots"
	}
	_ = os.MkdirAll(saveDir, 0755)
	return &BrowserService{
		saveDir:  saveDir,
		headless: true,
		timeout:  45 * time.Second,
	}
}

// OpenURL 打开页面并返回标题与文本摘要
func (s *BrowserService) OpenURL(ctx context.Context, url string) (title, text string, err error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", "", fmt.Errorf("url required")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	err = s.withChrome(ctx, func(ctx context.Context) error {
		var t, body string
		if err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Title(&t),
			chromedp.Text("body", &body, chromedp.ByQuery),
		); err != nil {
			return err
		}
		title = t
		text = body
		return nil
	})
	if err == nil {
		return title, truncateRunes(text, 4000), nil
	}

	// HTTP 降级
	return s.httpFallback(ctx, url)
}

// NavigateAndScreenshot 打开页面并截图
func (s *BrowserService) NavigateAndScreenshot(ctx context.Context, url string) (path string, err error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("url required")
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	out := filepath.Join(s.saveDir, fmt.Sprintf("browser_%d.png", time.Now().UnixMilli()))
	err = s.withChrome(ctx, func(ctx context.Context) error {
		var buf []byte
		if err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.FullScreenshot(&buf, 90),
		); err != nil {
			return err
		}
		return os.WriteFile(out, buf, 0644)
	})
	if err != nil {
		return "", fmt.Errorf("browser screenshot: %w (需本机安装 Chrome/Chromium)", err)
	}
	return out, nil
}

// EvalJS 在页面执行 JS 并返回结果字符串
func (s *BrowserService) EvalJS(ctx context.Context, url, script string) (string, error) {
	if script == "" {
		return "", fmt.Errorf("script required")
	}
	if url == "" {
		url = "about:blank"
	}
	var result interface{}
	err := s.withChrome(ctx, func(ctx context.Context) error {
		tasks := chromedp.Tasks{}
		if url != "about:blank" {
			tasks = append(tasks, chromedp.Navigate(url), chromedp.WaitReady("body", chromedp.ByQuery))
		}
		tasks = append(tasks, chromedp.Evaluate(script, &result))
		return chromedp.Run(ctx, tasks)
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprint(result), nil
}

// GetHTML 获取页面 HTML（长度截断）
func (s *BrowserService) GetHTML(ctx context.Context, url string) (string, error) {
	var html string
	err := s.withChrome(ctx, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		)
	})
	if err != nil {
		// HTTP 降级
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, e2 := http.DefaultClient.Do(req)
		if e2 != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 200000))
		return truncateRunes(string(b), 8000), nil
	}
	return truncateRunes(html, 8000), nil
}

func (s *BrowserService) withChrome(parent context.Context, fn func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", s.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	defer chromeCancel()

	return fn(chromeCtx)
}

func (s *BrowserService) httpFallback(ctx context.Context, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "AI-Desktop-Assistant/1.0")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("browser open failed (chrome+http): %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 100000))
	body := string(b)
	title := extractHTMLTitle(body)
	return title, truncateRunes(stripTags(body), 4000), nil
}

func extractHTMLTitle(html string) string {
	low := strings.ToLower(html)
	i := strings.Index(low, "<title>")
	if i < 0 {
		return ""
	}
	j := strings.Index(low[i:], "</title>")
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(html[i+7 : i+j])
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
