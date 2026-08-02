package enums

type IntentType string

const (
	// 文件操作
	IntentReadFile   IntentType = "READ_FILE"
	IntentWriteFile  IntentType = "WRITE_FILE"
	IntentListFiles  IntentType = "LIST_FILES"
	IntentDeleteFile IntentType = "DELETE_FILE"
	IntentCreateDir  IntentType = "CREATE_DIR"

	// 命令执行
	IntentRunCommand  IntentType = "RUN_COMMAND"
	IntentRunScript   IntentType = "RUN_SCRIPT"
	IntentStartApp    IntentType = "START_APP"

	// 屏幕截图
	IntentScreenshot  IntentType = "SCREENSHOT"
	IntentAnalyzeScreen IntentType = "ANALYZE_SCREEN"

	// 浏览器自动化
	IntentOpenUrl     IntentType = "OPEN_URL"
	IntentBrowserAction IntentType = "BROWSER_ACTION"

	// 系统操作
	IntentGetClipboard IntentType = "GET_CLIPBOARD"
	IntentSetClipboard IntentType = "SET_CLIPBOARD"
	IntentSystemInfo  IntentType = "SYSTEM_INFO"

	// 对话
	IntentChat        IntentType = "CHAT"
	IntentTaskPlan    IntentType = "TASK_PLAN"
	IntentUnknown     IntentType = "UNKNOWN"
)

func (i IntentType) Label() string {
	labels := map[IntentType]string{
		IntentReadFile: "读取文件",
		IntentWriteFile: "写入文件",
		IntentListFiles: "列出文件",
		IntentDeleteFile: "删除文件",
		IntentCreateDir: "创建目录",
		IntentRunCommand: "执行命令",
		IntentRunScript: "运行脚本",
		IntentStartApp: "启动应用",
		IntentScreenshot: "屏幕截图",
		IntentAnalyzeScreen: "分析屏幕",
		IntentOpenUrl: "打开网页",
		IntentBrowserAction: "浏览器操作",
		IntentGetClipboard: "获取剪贴板",
		IntentSetClipboard: "设置剪贴板",
		IntentSystemInfo: "系统信息",
		IntentChat: "对话",
		IntentTaskPlan: "任务规划",
		IntentUnknown: "未知",
	}
	return labels[i]
}

func (i IntentType) IsFileOperation() bool {
	return i == IntentReadFile || i == IntentWriteFile || i == IntentListFiles ||
		i == IntentDeleteFile || i == IntentCreateDir
}

func (i IntentType) IsCommandExecution() bool {
	return i == IntentRunCommand || i == IntentRunScript || i == IntentStartApp
}

func (i IntentType) RequiresPath() bool {
	return i == IntentReadFile || i == IntentWriteFile || i == IntentDeleteFile
}

func (i IntentType) RequiresContent() bool {
	return i == IntentWriteFile || i == IntentRunCommand || i == IntentRunScript
}
