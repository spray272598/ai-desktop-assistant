package intent

import (
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

type IntentRule struct {
	Intent   string
	Keywords []string
	Weight   float64
}

type RuleClassifier struct {
	rules []IntentRule
}

func NewRuleClassifier() *RuleClassifier {
	return &RuleClassifier{rules: initRules()}
}

func (c *RuleClassifier) Classify(input string, conv *valobj.ConversationContextVO) *valobj.IntentResult {
	input = strings.TrimSpace(input)
	if input == "" {
		return valobj.NewIntentResult(string(enums.IntentChat), 0.5, nil)
	}

	lower := strings.ToLower(input)
	bestIntent := string(enums.IntentChat)
	bestScore := 0.0
	var matchedKW string

	for _, rule := range c.rules {
		for _, kw := range rule.Keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(lower, kwLower) {
				// 更长关键词优先
				score := rule.Weight + float64(len(kw))*0.01
				if score > bestScore {
					bestScore = score
					bestIntent = rule.Intent
					matchedKW = kw
				}
			}
		}
	}

	if bestScore < 0.5 {
		// 指代消解：短回复沿用上轮意图
		if conv != nil && conv.LastIntent != "" && isReferential(lower) {
			entities := copyEntities(conv.EntityMemory)
			result := valobj.NewIntentResult(conv.LastIntent, 0.75, entities)
			result.Source = "rule-coref"
			return result
		}
		result := valobj.NewIntentResult(string(enums.IntentChat), 0.3, nil)
		result.Source = "rule"
		return result
	}

	entities := extractEntities(input, bestIntent)
	// 合并会话实体
	if conv != nil {
		for k, v := range conv.EntityMemory {
			if entities[k] == "" {
				entities[k] = v
			}
		}
	}
	confidence := 0.85
	if bestScore >= 1.0 {
		confidence = 0.92
	}
	_ = matchedKW
	result := valobj.NewIntentResult(bestIntent, confidence, entities)
	result.Source = "rule"
	return result
}

func initRules() []IntentRule {
	return []IntentRule{
		{Intent: "LIST_FILES", Keywords: []string{"列出文件", "列出目录", "查看目录", "目录下", "ls ", "list files", "dir ", "有哪些文件"}, Weight: 1.0},
		{Intent: "READ_FILE", Keywords: []string{"读取文件", "查看文件", "打开文件", "文件内容", "read file", "cat "}, Weight: 1.0},
		{Intent: "WRITE_FILE", Keywords: []string{"写入文件", "保存文件", "创建文件", "write file", "保存到"}, Weight: 1.0},
		{Intent: "DELETE_FILE", Keywords: []string{"删除文件", "移除文件", "delete file", "rm "}, Weight: 1.0},
		{Intent: "CREATE_DIR", Keywords: []string{"创建目录", "新建文件夹", "mkdir", "新建目录"}, Weight: 1.0},
		{Intent: "RUN_COMMAND", Keywords: []string{"执行命令", "运行命令", "执行cmd", "run command", "shell"}, Weight: 1.0},
		{Intent: "RUN_SCRIPT", Keywords: []string{"运行脚本", "执行脚本", "run script"}, Weight: 1.0},
		{Intent: "START_APP", Keywords: []string{"启动应用", "打开应用", "launch app", "start app"}, Weight: 0.9},
		{Intent: "SCREENSHOT", Keywords: []string{"截图", "截屏", "screenshot", "capture screen"}, Weight: 1.0},
		{Intent: "OPEN_URL", Keywords: []string{"打开网站", "打开网页", "open url", "浏览"}, Weight: 0.9},
		{Intent: "SYSTEM_INFO", Keywords: []string{"系统信息", "内存使用", "cpu", "system info"}, Weight: 0.9},
		{Intent: "TASK_PLAN", Keywords: []string{"帮我规划", "制定计划", "任务分解", "分步骤"}, Weight: 0.85},
	}
}

func isReferential(lower string) bool {
	refs := []string{"它", "这个", "那个", "继续", "再来", "同样", "同上", "再执行", "再做一次", "it", "same", "again", "continue"}
	for _, r := range refs {
		if strings.Contains(lower, r) {
			return true
		}
	}
	// 极短回复可能是指代
	return len([]rune(lower)) <= 6
}

func extractEntities(input, intent string) map[string]string {
	entities := make(map[string]string)
	switch intent {
	case "READ_FILE", "WRITE_FILE", "DELETE_FILE", "LIST_FILES", "CREATE_DIR":
		if p := extractAfterKeywords(input, []string{"读取文件", "查看文件", "打开文件", "写入文件", "保存文件", "删除文件", "创建目录", "列出", "读取", "查看", "打开", "写入", "保存", "删除"}); p != "" {
			entities["path"] = cleanPath(p)
		}
	case "RUN_COMMAND":
		if c := extractAfterKeywords(input, []string{"执行命令", "运行命令", "执行", "运行", "run", "execute"}); c != "" {
			entities["command"] = c
		}
	case "START_APP":
		if a := extractAfterKeywords(input, []string{"启动应用", "打开应用", "启动", "打开", "launch", "start"}); a != "" {
			entities["app"] = a
		}
	case "RUN_SCRIPT":
		if p := extractAfterKeywords(input, []string{"运行脚本", "执行脚本", "脚本"}); p != "" {
			entities["scriptPath"] = cleanPath(p)
		}
	}
	return entities
}

func extractAfterKeywords(input string, keywords []string) string {
	lower := strings.ToLower(input)
	for _, kw := range keywords {
		idx := strings.Index(lower, strings.ToLower(kw))
		if idx >= 0 {
			rest := strings.TrimSpace(input[idx+len(kw):])
			// 去掉引导词
			rest = strings.TrimPrefix(rest, "：")
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				// 只取第一段（空格/逗号前）
				for _, sep := range []string{"，", ",", " 的", " 并", " 然后"} {
					if i := strings.Index(rest, sep); i > 0 {
						rest = rest[:i]
					}
				}
				return strings.TrimSpace(rest)
			}
		}
	}
	return ""
}

func cleanPath(p string) string {
	p = strings.Trim(p, "\"'` ")
	return p
}

func copyEntities(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
