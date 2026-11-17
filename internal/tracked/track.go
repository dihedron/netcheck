package tracked

type Value[T any] struct {
	value    T
	accessed bool
}

func New[T any](value T) Value[T] {
	return Value[T]{
		value: value,
	}
}

func (g *Value[T]) Value() T {
	g.accessed = true
	return g.value
}

func (g *Value[T]) Accessed() bool {
	return g.accessed
}
