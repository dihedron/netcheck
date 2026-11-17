package fetch

import (
	"log/slog"
	"testing"

	"github.com/dihedron/netcheck/v2/internal/format"
)

func TestFromFile(t *testing.T) {

	tests := map[string]format.Format{
		"../../test/fixtures/netcheck.json": format.JSON,
		"../../test/fixtures/netcheck.yaml": format.YAML,
	}

	for path, expected := range tests {
		slog.Debug("reading bundle from file", "path", path)
		_, actual, err := FromFile(path)
		if err != nil {
			t.Fatalf("Could not read file %s: %v", path, err)
		}
		if expected != actual {
			t.Fatalf("Invalid format detected for file %s: expected %v, actual %v", path, expected, actual)
		}
	}
}
