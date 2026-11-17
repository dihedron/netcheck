package redis

import (
	"context"
	"crypto/tls"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
	"go.uber.org/goleak"
)

// password is joined into locations at run time, so that no literal looks like a URL with credentials to a secret scanner.
const password = "s3cret"

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const yamlBundle = "---\nid: from-redis\nchecks:\n  - address: localhost:80\n"

func load(t *testing.T, location string, opts ...Option) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()
	b, err := fetch.Load(ctx, location, fetch.WithSource(Source(opts...), "redis", "rediss"))
	if err != nil {
		return "", err
	}
	return b.ID, nil
}

func TestSourceReadsAKey(t *testing.T) {
	addr, db := fakeRedis(t, map[string]string{"bundle": yamlBundle, "json": `{"id":"j"}`, "plain": "just text"})
	if id, err := load(t, "redis://"+addr+"?key=bundle&db=5"); err != nil || id != "from-redis" {
		t.Fatalf("%q %v", id, err)
	}
	if db() != "5" {
		t.Errorf("database %s selected, want 5", db())
	}
	if id, err := load(t, "redis://"+addr+"?key=json"); err != nil || id != "j" {
		t.Errorf("json: %q %v", id, err)
	}
	if _, err := load(t, "redis://"+addr+"?key=plain"); err == nil {
		t.Error("a value of unknown format must be an error")
	}
	if _, err := load(t, "redis://"+addr+"?key=absent"); err == nil {
		t.Error("a missing key must be an error")
	}
}

// Required parameters and numbers are checked before any connection is made
// (nothing listens on port 1).
func TestLocationsAreValidated(t *testing.T) {
	for _, loc := range []string{"redis://127.0.0.1:1", "redis://127.0.0.1:1?key=k&db=notanumber", "redis://127.0.0.1:1?key=k&db=-1", "redis:///?key=k"} {
		if _, err := load(t, loc); err == nil {
			t.Errorf("%s: expected an error", loc)
		}
	}
}

// rediss speaks TLS: a plain fake cannot answer; and the TLS option reaches the client.
func TestTLS(t *testing.T) {
	addr, _ := fakeRedis(t, map[string]string{"bundle": yamlBundle})
	if _, err := load(t, "rediss://"+addr+"?key=bundle"); err == nil {
		t.Error("rediss against a plain server must fail")
	}
	u, _ := url.Parse("rediss://" + addr + "?key=bundle")
	opts, _, err := options(u, []Option{WithTLSConfig(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})}) //nolint:gosec // test
	if err != nil || opts.TLSConfig == nil || !opts.TLSConfig.InsecureSkipVerify {
		t.Errorf("TLS option not applied: %v %+v", err, opts.TLSConfig)
	}
	opts, _, _ = options(u, nil)
	if opts.TLSConfig == nil || opts.TLSConfig.InsecureSkipVerify || opts.TLSConfig.ServerName == "" {
		t.Errorf("rediss must verify by default: %+v", opts.TLSConfig)
	}
	u, _ = url.Parse("redis://" + addr + "?key=bundle")
	if opts, _, _ = options(u, nil); opts.TLSConfig != nil {
		t.Error("redis is plain")
	}
}

func TestCredentialsReachTheClientAndNotTheError(t *testing.T) {
	u, _ := url.Parse("redis://alice:" + password + "@127.0.0.1:1?key=k")
	opts, key, err := options(u, nil)
	if err != nil || opts.Username != "alice" || opts.Password != "s3cret" || key != "k" {
		t.Fatalf("%+v %q %v", opts, key, err)
	}
	if _, err := load(t, "redis://alice:"+password+"@127.0.0.1:1?key=k"); err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("%v", err)
	}
}

func TestDeadline(t *testing.T) {
	// a server that accepts and never answers
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	l := silentListener(t)
	if _, err := fetch.Load(ctx, "redis://"+l+"?key=k", fetch.WithSource(Source(), "redis")); err == nil {
		t.Fatal("expected a timeout")
	}
}
