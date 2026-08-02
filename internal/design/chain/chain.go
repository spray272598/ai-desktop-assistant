package chain

import "fmt"

type Handler[T any] interface {
	Handle(ctx T) error
	Name() string
}

type Chain[T any] struct {
	handlers []Handler[T]
}

func NewChain[T any](handlers ...Handler[T]) *Chain[T] {
	return &Chain[T]{handlers: handlers}
}

func (c *Chain[T]) Handle(ctx T) error {
	for _, handler := range c.handlers {
		if err := handler.Handle(ctx); err != nil {
			return fmt.Errorf("handler [%s] failed: %w", handler.Name(), err)
		}
	}
	return nil
}

func (c *Chain[T]) AddHandler(handler Handler[T]) *Chain[T] {
	c.handlers = append(c.handlers, handler)
	return c
}
