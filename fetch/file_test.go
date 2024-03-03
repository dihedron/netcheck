package fetch

import (
	"log"
	"log/slog"
	"testing"

	"github.com/dihedron/netcheck/format"
)

func TestFromFile(t *testing.T) {

	tests := map[string]format.Format{
		"../_test/netcheck.json": format.JSON,
		"../_test/netcheck.yaml": format.YAML,
		"../_test/netcheck.toml": format.TOML,
	}

	for path, expected := range tests {
		slog.Debug("reading bundle from file", "path", path)
		_, actual, err := FromFile(path)
		if err != nil {
			log.Fatalf("Could not read file %s: %v", path, err)
		}
		if expected != actual {
			log.Fatalf("Invalid format detecte for file %s: expected %v, actual %v", path, expected, actual)
		}
	}
}
