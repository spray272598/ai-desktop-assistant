package provider

// ContextProvider 上下文提供者接口（对标 walicode ContextProvider）
type ContextProvider interface {
	Name() string
	Order() int
	Enabled() bool
	Provide(sessionID, userID, workingDir string, messageHistory []map[string]interface{}) map[string]interface{}
}
