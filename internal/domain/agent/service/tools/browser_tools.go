package tools

import (
	"context"
	"fmt"

	"github.com/ai-desktop/assistant/internal/domain/desktop/service"
)

type OpenURLTool struct {
	browser *service.BrowserService
}

func NewOpenURLTool(b *service.BrowserService) *OpenURLTool { return &OpenURLTool{browser: b} }
func (t *OpenURLTool) Name() string                         { return "open_url" }
func (t *OpenURLTool) Description() string {
	return "打开网页并返回页面标题与正文摘要（浏览器自动化）"
}
func (t *OpenURLTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	url := getStringArg(args, "url")
	if url == "" {
		url = getStringArg(args, "path")
	}
	if url == "" {
		return "错误: 需要参数 url", nil
	}
	title, text, err := t.browser.OpenURL(ctx, url)
	if err != nil {
		return fmt.Sprintf("打开失败: %v", err), nil
	}
	return fmt.Sprintf("标题: %s\nURL: %s\n\n正文摘要:\n%s", title, url, text), nil
}

type BrowserScreenshotTool struct {
	browser *service.BrowserService
}

func NewBrowserScreenshotTool(b *service.BrowserService) *BrowserScreenshotTool {
	return &BrowserScreenshotTool{browser: b}
}
func (t *BrowserScreenshotTool) Name() string { return "browser_screenshot" }
func (t *BrowserScreenshotTool) Description() string {
	return "打开指定 URL 并截取整页截图，返回保存路径"
}
func (t *BrowserScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	url := getStringArg(args, "url")
	if url == "" {
		return "错误: 需要参数 url", nil
	}
	path, err := t.browser.NavigateAndScreenshot(ctx, url)
	if err != nil {
		return fmt.Sprintf("截图失败: %v", err), nil
	}
	return "✅ 浏览器截图已保存: " + path, nil
}

type BrowserEvalTool struct {
	browser *service.BrowserService
}

func NewBrowserEvalTool(b *service.BrowserService) *BrowserEvalTool {
	return &BrowserEvalTool{browser: b}
}
func (t *BrowserEvalTool) Name() string { return "browser_eval" }
func (t *BrowserEvalTool) Description() string {
	return "在页面上下文执行 JavaScript，参数: url, script"
}
func (t *BrowserEvalTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	url := getStringArg(args, "url")
	script := getStringArg(args, "script")
	if script == "" {
		return "错误: 需要参数 script", nil
	}
	res, err := t.browser.EvalJS(ctx, url, script)
	if err != nil {
		return fmt.Sprintf("执行失败: %v", err), nil
	}
	return res, nil
}

type BrowserHTMLTool struct {
	browser *service.BrowserService
}

func NewBrowserHTMLTool(b *service.BrowserService) *BrowserHTMLTool {
	return &BrowserHTMLTool{browser: b}
}
func (t *BrowserHTMLTool) Name() string { return "browser_html" }
func (t *BrowserHTMLTool) Description() string {
	return "获取网页 HTML 源码摘要，参数: url"
}
func (t *BrowserHTMLTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	url := getStringArg(args, "url")
	if url == "" {
		return "错误: 需要参数 url", nil
	}
	html, err := t.browser.GetHTML(ctx, url)
	if err != nil {
		return fmt.Sprintf("获取失败: %v", err), nil
	}
	return html, nil
}
