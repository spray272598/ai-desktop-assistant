package exception

import "fmt"

type AppException struct {
	Code    string
	Message string
	Err     error
}

func NewAppException(code, message string) *AppException {
	return &AppException{Code: code, Message: message}
}

func NewAppExceptionWithError(code, message string, err error) *AppException {
	return &AppException{Code: code, Message: message, Err: err}
}

func (e *AppException) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
