package prompt

import (
	"fmt"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// PromptService 提示词服务
type PromptService struct {
	builder *DynamicPromptBuilder
}

func NewPromptService() *PromptService {
	return &PromptService{builder: NewDynamicPromptBuilder()}
}

func (s *PromptService) BaseSystemInstruction(agentName string) string {
	if agentName == "" {
		agentName = "AI桌面助手"
	}
	var sb strings.Builder
	sb.WriteString("你是一个强大的桌面AI助手，名叫")
	sb.WriteString(agentName)
	sb.WriteString("。你可以帮助用户完成以下任务：\n\n")
	sb.WriteString("1. **文件操作**: 读取、写入、删除文件，列出目录\n")
	sb.WriteString("2. **命令执行**: 运行Shell命令、执行脚本\n")
	sb.WriteString("3. **屏幕截图**: 截取屏幕\n")
	sb.WriteString("4. **系统操作**: 获取系统信息\n\n")
	sb.WriteString("## 工作原则\n")
	sb.WriteString("- 总是先理解用户意图，再选择合适的工具\n")
	sb.WriteString("- 每一步操作前都要思考并说明原因\n")
	sb.WriteString("- 操作完成后给出清晰的结果反馈\n")
	sb.WriteString("- 遇到不确定的情况，主动向用户确认\n")
	sb.WriteString("- 保持安全意识，避免执行危险操作\n")
	sb.WriteString("- 工作目录受限，文件操作仅限工作区内\n")
	return sb.String()
}

func (s *PromptService) BuildSystemPrompt(ctx *valobj.PromptContextVO) string {
	name := ""
	if ctx != nil {
		name = ctx.AgentName
	}
	base := s.BaseSystemInstruction(name)
	return s.builder.BuildSystem(base, ctx)
}

func (s *PromptService) BuildUserPrompt(userInput string, ctx *valobj.PromptContextVO) string {
	prefix := s.builder.BuildMessagePrefix(ctx)
	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(prefix)
		sb.WriteString("\n")
	}
	sb.WriteString("## 用户请求\n")
	sb.WriteString(userInput)
	return sb.String()
}

func (s *PromptService) BuildStepPrompt(step int, userInput string) string {
	return fmt.Sprintf("【第 %d 步】基于工具结果继续推理。用户原始请求：%s\n若已完成请直接回答；若需更多工具请输出 JSON 工具调用。", step, userInput)
}

func (s *PromptService) Builder() *DynamicPromptBuilder {
	return s.builder
}
