package enums

type ResponseCode struct {
	Code    string
	Message string
}

var (
	Success             = ResponseCode{Code: "0000", Message: "success"}
	SystemError         = ResponseCode{Code: "5000", Message: "system error"}
	InvalidParam        = ResponseCode{Code: "4000", Message: "invalid parameter"}
	SessionNotFound     = ResponseCode{Code: "4001", Message: "session not found"}
	FileNotFound        = ResponseCode{Code: "4101", Message: "file not found"}
	CommandFailed       = ResponseCode{Code: "5101", Message: "command execution failed"}
	LLMCallFailed       = ResponseCode{Code: "5201", Message: "LLM call failed"}
	ToolNotAvailable    = ResponseCode{Code: "5301", Message: "tool not available"}
)
