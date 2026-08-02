package tree

import "fmt"

type Handler[T any, R any] interface {
	Apply(ctx T) (R, error)
	Name() string
}

type Tree[T any, R any] struct {
	handlers map[string]Handler[T, R]
}

func NewTree[T any, R any]() *Tree[T, R] {
	return &Tree[T, R]{handlers: make(map[string]Handler[T, R])}
}

func (t *Tree[T, R]) Register(key string, handler Handler[T, R]) *Tree[T, R] {
	t.handlers[key] = handler
	return t
}

func (t *Tree[T, R]) Apply(key string, ctx T) (R, error) {
	handler, ok := t.handlers[key]
	if !ok {
		var zero R
		return zero, fmt.Errorf("handler not found for key: %s", key)
	}
	return handler.Apply(ctx)
}

func (t *Tree[T, R]) HasHandler(key string) bool {
	_, ok := t.handlers[key]
	return ok
}

func (t *Tree[T, R]) Keys() []string {
	keys := make([]string, 0, len(t.handlers))
	for k := range t.handlers {
		keys = append(keys, k)
	}
	return keys
}
