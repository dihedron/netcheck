package pointer

// To returns the pointer to the given value.
func To[T any](value T) *T {
	return &value
}
