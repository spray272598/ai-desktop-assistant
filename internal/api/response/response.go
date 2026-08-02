package response

import "github.com/ai-desktop/assistant/internal/types/enums"

type Response[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

func Success[T any](data T) *Response[T] {
	return &Response[T]{
		Code:    enums.Success.Code,
		Message: enums.Success.Message,
		Data:    data,
	}
}

func Error(code string, message string) *Response[any] {
	return &Response[any]{
		Code:    code,
		Message: message,
	}
}

func ErrorFromCode(code enums.ResponseCode) *Response[any] {
	return &Response[any]{
		Code:    code.Code,
		Message: code.Message,
	}
}
