package logging

import (
	"strings"
	"testing"
)

func TestDumps(t *testing.T) {
	v := map[string]int{"a": 1}
	if ToJSON(v) != `{"a":1}` {
		t.Errorf("ToJSON: %s", ToJSON(v))
	}
	if !strings.Contains(ToPrettyJSON(v), "\n  \"a\": 1") {
		t.Errorf("ToPrettyJSON: %s", ToPrettyJSON(v))
	}
	if ToYAML(v) != "a: 1\n" {
		t.Errorf("ToYAML: %q", ToYAML(v))
	}
}
