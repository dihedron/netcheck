package fetch

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// fileSource reads a bundle from the file system; the format hint comes from
// the extension.
type fileSource struct{}

func (fileSource) Fetch(ctx context.Context, u *url.URL, maxSize int64) ([]byte, Format, error) {
	if err := ctx.Err(); err != nil {
		return nil, Unknown, err
	}
	f, err := os.Open(filepath.Clean(u.Path)) //nolint:gosec // reading the file the caller named is the point
	if err != nil {
		return nil, Unknown, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, Unknown, fmt.Errorf("reading %s: %w", u.Path, err)
	}
	return data, hintFromPath(u.Path), nil
}

func hintFromPath(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return YAML
	case ".json":
		return JSON
	}
	return Unknown
}
