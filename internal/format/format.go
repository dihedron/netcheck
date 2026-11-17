package format

import (
	"errors"
	"log/slog"
	"strings"
)

type Format int8

const (
	YAML Format = iota
	JSON
)

// Detect tries to detect the data format in the given file.
func Detect(data string) (Format, error) {
	slog.Debug("detecting data format from string...", "data", data)
	trimmed := strings.TrimLeft(data, "\n\r\t")
	if strings.HasPrefix(trimmed, "---") {
		slog.Debug("format is YAML")
		return YAML, nil
	} else if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		slog.Debug("format is JSON")
		return JSON, nil
	}
	return Format(-1), errors.New("unsupported or undetected format")
}
