package common

const (
	SessionActive    = "ACTIVE"
	SessionCompleted = "COMPLETED"
	SessionExpired   = "EXPIRED"

	DefaultMaxSteps    = 15
	DefaultTokenBudget = 16000
	DefaultWindowSize  = 30
	DefaultTimeout     = 60

	// DefaultListActiveLimit 活跃会话列表默认条数
	DefaultListActiveLimit = 50
	// DefaultFindByUserLimit 用户会话列表默认条数
	DefaultFindByUserLimit = 100

	DefaultWorkingDir = "./workspace"
	TempDir           = "./temp"
)

// EstimateTokens 粗略估算 token 数（中英混合近似：rune 数 * 0.5，至少 1）
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	n := len([]rune(text))
	t := n / 2
	if t < 1 {
		return 1
	}
	return t
}

