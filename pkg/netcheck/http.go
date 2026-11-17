package netcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	maxRedirects = 10
	maxBodyBytes = 64 << 10
)

// httpRunner passes when the final response, after at most 10 redirects, has a
// status from 200 to 399. It never uses or changes http.DefaultClient.
type httpRunner struct{ base *http.Client }

func (h httpRunner) Run(ctx context.Context, c Check, _ Defaults) (Detail, error) {
	if c.SSO {
		return Detail{}, errors.New("sso needs a runner from the sso sub-package: register it with WithRunner")
	}
	if _, err := dialTarget(c.Protocol, c.Address); err != nil {
		return Detail{}, err
	}
	client, err := h.client(c)
	if err != nil {
		return Detail{}, err
	}
	url := string(c.Protocol) + "://" + c.Address
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return Detail{}, fmt.Errorf("building the request for %s: %w", redact(c.Address), err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Detail{}, fmt.Errorf("requesting %s: %w", redact(url), err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))

	detail := Detail{HTTPStatus: resp.StatusCode}
	if resp.Request != nil && resp.Request.URL != nil {
		detail.Remote = resp.Request.URL.Host
	}
	if resp.TLS != nil {
		detail.TLS = factsFromState(*resp.TLS, c.TLS == nil || !c.TLS.InsecureSkipVerify)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return detail, fmt.Errorf("%s answered %s", redact(url), resp.Status)
	}
	return detail, nil
}

// client returns a client for this check: a copy of the base client, with a
// cloned transport when the check has TLS options, and the redirect policy.
func (h httpRunner) client(c Check) (*http.Client, error) {
	base := h.base
	if base == nil {
		base = &http.Client{Transport: http.DefaultTransport}
	}
	client := *base
	if c.TLS != nil {
		t, ok := client.Transport.(*http.Transport)
		if client.Transport == nil {
			t, ok = http.DefaultTransport.(*http.Transport), true
		}
		if !ok {
			return nil, errors.New("TLS options need an *http.Transport in the HTTP client")
		}
		t = t.Clone()
		t.TLSClientConfig = tlsConfig(c.TLS, "")
		t.TLSClientConfig.ServerName = c.TLS.ServerName // empty: derived from the URL
		client.Transport = t
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		if via[0].URL.Scheme == "https" && req.URL.Scheme == "http" && !c.AllowDowngrade {
			return errors.New("redirect from https to http refused (downgrade)")
		}
		return nil
	}
	return &client, nil
}
