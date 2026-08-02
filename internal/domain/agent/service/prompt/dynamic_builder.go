package prompt

import (
	"fmt"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// DynamicPromptBuilder 动态 Prompt 组装器（对标 walicode DynamicPromptBuilder）
type DynamicPromptBuilder struct{}

func NewDynamicPromptBuilder() *DynamicPromptBuilder {
	return &DynamicPromptBuilder{}
}

// BuildSystem 在基础 system 指令上叠加动态上下文
func (b *DynamicPromptBuilder) BuildSystem(baseInstruction string, ctx *valobj.PromptContextVO) string {
	if ctx == nil {
		return baseInstruction
	}
	var sb strings.Builder
	sb.WriteString(baseInstruction)
	b.appendEnvironment(&sb, ctx)
	b.appendTools(&sb, ctx)
	return sb.String()
}

// BuildMessagePrefix 注入到用户消息前的动态前缀（区分实时状态 vs 历史上下文）
func (b *DynamicPromptBuilder) BuildMessagePrefix(ctx *valobj.PromptContextVO) string {
	if ctx == nil {
		return ""
	}
	var sb strings.Builder
	hasContent := false

	// 实时状态
	if ctx.OsInfo != "" || ctx.CurrentDir != "" || ctx.ServerInfo != "" || ctx.UserID != "" {
		sb.WriteString("[系统环境]\n")
		if ctx.ServerInfo != "" {
			sb.WriteString("服务器: " + ctx.ServerInfo + "\n")
		}
		if ctx.OsInfo != "" {
			sb.WriteString("系统: " + ctx.OsInfo + "\n")
		}
		if ctx.UserID != "" {
			sb.WriteString("用户: " + ctx.UserID + "\n")
		}
		if ctx.CurrentDir != "" {
			sb.WriteString("目录: " + ctx.CurrentDir + "\n")
		}
		hasContent = true
	}

	if ctx.ProjectName != "" || ctx.ProjectRootPath != "" {
		sb.WriteString("\n[当前工程]\n")
		if ctx.ProjectName != "" {
			sb.WriteString("工程名: " + ctx.ProjectName + "\n")
		}
		if ctx.ProjectRootPath != "" {
			sb.WriteString("工程路径: " + ctx.ProjectRootPath + "\n")
		}
		hasContent = true
	}

	if ctx.IntentResult != nil && ctx.IntentResult.IsAction {
		sb.WriteString(fmt.Sprintf("\n[意图识别] %s (置信度 %.2f, 来源 %s)\n",
			ctx.IntentResult.Intent, ctx.IntentResult.Confidence, ctx.IntentResult.Source))
		if len(ctx.IntentResult.Entities) > 0 {
			sb.WriteString("实体: ")
			for k, v := range ctx.IntentResult.Entities {
				sb.WriteString(fmt.Sprintf("%s=%s ", k, v))
			}
			sb.WriteString("\n")
		}
		hasContent = true
	}

	// 历史上下文（标注避免误导）
	hasHist := false
	appendHistHeader := func() {
		if !hasHist {
			sb.WriteString("\n[历史对话记录 — 以下信息来自之前对话，请以当前用户消息的意图为主]\n")
			hasHist = true
		}
	}

	if len(ctx.RecentCommands) > 0 {
		appendHistHeader()
		sb.WriteString("最近执行的命令:\n")
		for _, cmd := range ctx.RecentCommands {
			sb.WriteString("- " + cmd + "\n")
		}
		hasContent = true
	}

	if len(ctx.Milestones) > 0 {
		appendHistHeader()
		sb.WriteString("关键事件:\n")
		ms := ctx.Milestones
		if len(ms) > 8 {
			ms = ms[len(ms)-8:]
		}
		for _, m := range ms {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", m.Type, m.Content))
		}
		hasContent = true
	}

	if ctx.ToolResultSummary != "" {
		appendHistHeader()
		sb.WriteString("工具执行摘要:\n")
		sb.WriteString(ctx.ToolResultSummary)
		if !strings.HasSuffix(ctx.ToolResultSummary, "\n") {
			sb.WriteString("\n")
		}
		hasContent = true
	}

	if ctx.TaskDescription != "" {
		appendHistHeader()
		sb.WriteString("任务描述: " + ctx.TaskDescription + "\n")
		hasContent = true
	}

	if ctx.CoreMemories != "" {
		appendHistHeader()
		sb.WriteString("核心记忆:\n")
		sb.WriteString(ctx.CoreMemories)
		sb.WriteString("\n")
		hasContent = true
	}

	if !hasContent {
		return ""
	}
	return sb.String()
}

func (b *DynamicPromptBuilder) appendEnvironment(sb *strings.Builder, ctx *valobj.PromptContextVO) {
	sb.WriteString("\n## 当前环境\n")
	if ctx.UserID != "" {
		sb.WriteString(fmt.Sprintf("- 用户: %s\n", ctx.UserID))
	}
	if ctx.CurrentDir != "" {
		sb.WriteString(fmt.Sprintf("- 工作目录: %s\n", ctx.CurrentDir))
	}
	if ctx.OsInfo != "" {
		sb.WriteString(fmt.Sprintf("- 操作系统: %s\n", ctx.OsInfo))
	}
	if ctx.ServerInfo != "" {
		sb.WriteString(fmt.Sprintf("- 主机: %s\n", ctx.ServerInfo))
	}
}

func (b *DynamicPromptBuilder) appendTools(sb *strings.Builder, ctx *valobj.PromptContextVO) {
	if len(ctx.AvailableTools) == 0 {
		return
	}
	sb.WriteString("\n## 可用工具\n")
	sb.WriteString("需要调用工具时，请仅输出如下 JSON（不要包裹 markdown）：\n")
	sb.WriteString(`{"name":"工具名","args":{...}}` + "\n\n")
	for _, tool := range ctx.AvailableTools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description))
		if len(tool.Parameters) > 0 {
			sb.WriteString("  参数: ")
			parts := make([]string, 0, len(tool.Parameters))
			for k, v := range tool.Parameters {
				parts = append(parts, k+"("+v+")")
			}
			sb.WriteString(strings.Join(parts, ", "))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n若任务已完成，直接用自然语言给出最终答案，不要输出工具 JSON。\n")
}
