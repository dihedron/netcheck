package tracked

import "testing"

func TestValueTracksAccess(t *testing.T) {
	v := New("x")
	if v.Accessed() {
		t.Fatal("a new value is not accessed")
	}
	if v.Value() != "x" || !v.Accessed() {
		t.Errorf("reading must return the value and mark it accessed")
	}
}

func TestPeekDoesNotTrack(t *testing.T) {
	v := New(7)
	if v.Peek() != 7 || v.Accessed() {
		t.Error("Peek must not mark the value as accessed")
	}
}
