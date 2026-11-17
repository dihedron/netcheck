package pointer

import "testing"

func TestTo(t *testing.T) {
	p := To(42)
	if *p != 42 {
		t.Errorf("got %d", *p)
	}
	q := To(42)
	if p == q {
		t.Error("each call returns a new pointer")
	}
}
