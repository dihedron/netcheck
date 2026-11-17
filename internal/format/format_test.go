package format

import "testing"

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Format
		err  bool
	}{
		{"---\nid: x\n", YAML, false},
		{"\n\r\t---\nid: x\n", YAML, false},
		{`{"id":"x"}`, JSON, false},
		{"\n[1]", JSON, false},
		{"id: x\n", Format(-1), true},
		{"", Format(-1), true},
	} {
		got, err := Detect(tc.in)
		if (err != nil) != tc.err || (!tc.err && got != tc.want) {
			t.Errorf("Detect(%q) = %v, %v; want %v (error %v)", tc.in, got, err, tc.want, tc.err)
		}
	}
}
