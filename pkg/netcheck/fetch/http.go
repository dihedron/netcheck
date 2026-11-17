package fetch

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
)

// httpSource reads a bundle over HTTP or HTTPS; the format hint comes from
// the Content-Type, then from the extension of the path.
type httpSource struct{ client *http.Client }

func (h httpSource) Fetch(ctx context.Context, u *url.URL, maxSize int64) ([]byte, Format, error) {
	client := h.client
	if client == nil {
		client = &http.Client{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, Unknown, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, Unknown, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, Unknown, fmt.Errorf("the server answered %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, Unknown, fmt.Errorf("reading the answer: %w", err)
	}
	hint := Unknown
	if media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err == nil {
		switch media {
		case "application/json":
			hint = JSON
		case "application/x-yaml", "application/yaml", "text/yaml", "text/x-yaml":
			hint = YAML
		}
	}
	if hint == Unknown {
		hint = hintFromPath(u.Path)
	}
	return data, hint, nil
}
