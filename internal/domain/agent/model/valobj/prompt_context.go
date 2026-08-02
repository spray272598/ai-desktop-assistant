package valobj

// PromptContextVO 动态提示词上下文
type PromptContextVO struct {
	AgentName         string
	UserID            string
	CurrentDir        string
	OsInfo            string
	ServerInfo        string
	RecentActions     []string
	RecentCommands    []string
	Milestones        []*MilestoneVO
	IntentResult      *IntentResult
	AvailableTools    []*ToolInfo
	UserInput         string
	ToolResultSummary string
	TaskDescription   string
	CoreMemories      string
	ProjectName       string
	ProjectRootPath   string
}

type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

func NewPromptContext(agentName, userID, currentDir string) *PromptContextVO {
	return &PromptContextVO{
		AgentName:      agentName,
		UserID:         userID,
		CurrentDir:     currentDir,
		RecentActions:  make([]string, 0),
		RecentCommands: make([]string, 0),
		Milestones:     make([]*MilestoneVO, 0),
		AvailableTools: make([]*ToolInfo, 0),
	}
}

func (p *PromptContextVO) AddRecentAction(action string) {
	p.RecentActions = append(p.RecentActions, action)
	if len(p.RecentActions) > 20 {
		p.RecentActions = p.RecentActions[len(p.RecentActions)-20:]
	}
}

func (p *PromptContextVO) AddMilestone(milestone *MilestoneVO) {
	p.Milestones = append(p.Milestones, milestone)
	if len(p.Milestones) > 50 {
		p.Milestones = p.Milestones[len(p.Milestones)-50:]
	}
}

func (p *PromptContextVO) AddTool(tool *ToolInfo) {
	p.AvailableTools = append(p.AvailableTools, tool)
}

func (p *PromptContextVO) SetTools(tools []*ToolInfo) {
	p.AvailableTools = tools
}
