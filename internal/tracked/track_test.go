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
