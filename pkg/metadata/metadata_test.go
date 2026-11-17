package metadata

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrint(t *testing.T) {
	VersionMajor, VersionMinor, VersionPatch, Description = "1", "2", "3", "test build"
	var b bytes.Buffer
	Print(&b)
	if !strings.Contains(b.String(), "v1.2.3") || !strings.Contains(b.String(), "test build") {
		t.Errorf("Print wrote %q", b.String())
	}
	PrintFull(&b) // writes to standard output
}
