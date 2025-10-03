package fetch

import (
	"bytes"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dihedron/netcheck/format"
	"github.com/dihedron/netcheck/logging"
	"github.com/dpotapov/go-spnego"
	// "github.com/dpotapov/go-spnego"
)

// FromHTTP retrieves a bundle from an HTTP URL; the server must set the Content-Type
// header correctly in order to give the right hint about which parser to use to
// read and analyse the checks bundle. If the URL has the "https-://" scheme, the
// certificate verification is skipped.
func FromHTTP(path string) ([]byte, format.Format, error) {

	// handle the case where TLS verification is disabled
	u, err := url.Parse(path)
	if err != nil {
		slog.Error("error parsing HTTP URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	slog.Debug("parsed HTTP URL", "object", logging.ToJSON(u))

	client := http.DefaultClient

	if u.Scheme == "http+sso" || u.Scheme == "https+sso" || u.Scheme == "https+sso-" {
		slog.Debug("SSO authentication requested...")
		// create an NTM-aware transport
		client.Transport = &spnego.Transport{}
		client.Transport.(*spnego.Transport).Transport = *http.DefaultTransport.(*http.Transport).Clone()
		if u.Scheme == "https+sso-" {
			slog.Debug("disabling TLS verification...")
			client.Transport.(*spnego.Transport).Transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true, // #nosec G402
			}
		}
		u.Scheme = strings.ReplaceAll(u.Scheme, "+sso", "")
		if u.Scheme == "https-" {
			u.Scheme = "https"
		}
	} else {
		slog.Debug("plain HTTP(s) requested...")
		if u.Scheme == "https-" {
			slog.Debug("disabling TLS verification...")
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // #nosec G402
				},
			}
			u.Scheme = "https"
		} else {
			slog.Debug("different scheme", "scheme", u.Scheme)
		}
	}

	path = u.String()
	slog.Debug("placing request to HTTP server", "url", path)

	resp, err := client.Get(path)
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
