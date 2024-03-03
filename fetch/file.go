package fetch

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dihedron/netcheck/format"
)

// FromFile
func FromFile(path string) ([]byte, format.Format, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("error reading package from file", "path", path, "error", err)
		return nil, format.Format(-1), err
	}
	var f format.Format
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		f = format.YAML
	case ".json":
		f = format.JSON
	case ".toml":
		f = format.TOML
	}
	return data, f, nil
}