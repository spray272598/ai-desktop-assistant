package valobj

type CommandType string

const (
	CmdShell   CommandType = "SHELL"
	CmdScript  CommandType = "SCRIPT"
	CmdProcess CommandType = "PROCESS"
)

type CommandOperation struct {
	Type    CommandType
	Command string
	Args    []string
	WorkDir string
	Env     map[string]string
	Timeout int
}

type CommandResult struct {
	Success  bool
	Stdout   string
	Stderr   string
	ExitCode int
	Duration int64
}
