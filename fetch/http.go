package fetch

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/dihedron/netcheck/format"
)

// FromHTTP retrieves a bundle from an HTTP URL; the server must set the Content-Type
// header correctly in order to give the right hint about which parser to use to
// read and analyse the checks bundle.
func FromHTTP(path string) ([]byte, format.Format, error) {

	resp, err := http.Get(path)
	if err != nil {
		slog.Error("error downloading bundle file from URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}
	defer resp.Body.Close()

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, resp.Body)
	if err != nil {
		slog.Error("error reading bundle file body from URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	var f format.Format
	switch resp.Header.Get("Content-Type") {
	case "application/json":
		f = format.JSON
	case "application/x-yaml", "text/yaml":
		f = format.YAML
	}

	slog.Debug("bundle file retrieved from HTTP(s) source", "path", path, "format", f, "data", buffer.String())
	return buffer.Bytes(), f, nil
}
