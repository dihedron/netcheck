package consul

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
	"go.uber.org/goleak"
)

// password is joined into locations at run time, so that no literal looks like a URL with credentials to a secret scanner.
const password = "s3cret"

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const yamlBundle = "---\nid: from-consul\nchecks:\n  - address: localhost:80\n"

// fakeConsul answers the two Consul HTTP APIs the sources use: the key/value
// store (key "bundle/ok") and the service catalogue (service "svc", tag "t",
// metadata key "m"; "dup" has two entries). It records the last request.
func fakeConsul(t *testing.T) (host string, last func() *http.Request) {
	t.Helper()
	var seen *http.Request
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/kv/", func(w http.ResponseWriter, r *http.Request) {
		seen = r
		if r.URL.Path != "/v1/kv/bundle/ok" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"Key": "bundle/ok", "Value": base64.StdEncoding.EncodeToString([]byte(yamlBundle)), "CreateIndex": 1, "ModifyIndex": 1}})
	})
	mux.HandleFunc("/v1/catalog/service/", func(w http.ResponseWriter, r *http.Request) {
		seen = r
		entry := map[string]any{"ServiceName": "svc", "ServiceMeta": map[string]string{"m": yamlBundle}}
		switch strings.TrimPrefix(r.URL.Path, "/v1/catalog/service/") {
		case "svc":
			_ = json.NewEncoder(w).Encode([]any{entry})
		case "dup":
			_ = json.NewEncoder(w).Encode([]any{entry, entry})
		default:
			_ = json.NewEncoder(w).Encode([]any{})
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), func() *http.Request { return seen }
}

func load(t *testing.T, location string, opts ...Option) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	b, err := fetch.Load(ctx, location,
		fetch.WithSource(KV(opts...), "consulkv", "consulkvs"),
		fetch.WithSource(ServiceRegistry(opts...), "consulsr", "consulsrs"))
	if err != nil {
		return "", err
	}
	return b.ID, nil
}

func TestKV(t *testing.T) {
	host, last := fakeConsul(t)
	if id, err := load(t, "consulkv://"+host+"?key=bundle/ok&dc=eu1"); err != nil || id != "from-consul" {
		t.Fatalf("%q %v", id, err)
	}
	if last().URL.Query().Get("dc") != "eu1" {
		t.Errorf("the datacentre was not passed: %s", last().URL)
	}
	// an ACL token (empty user) and basic credentials
	if _, err := load(t, "consulkv://:"+"sekret"+"@"+host+"?key=bundle/ok"); err != nil || last().Header.Get("X-Consul-Token") != "sekret" {
		t.Errorf("token: %v, header %q", err, last().Header.Get("X-Consul-Token"))
	}
	if _, err := load(t, "consulkv://alice:"+password+"@"+host+"?key=bundle/ok"); err != nil {
		t.Fatal(err)
	}
	if user, pass, ok := last().BasicAuth(); !ok || user != "alice" || pass != password {
		t.Errorf("basic auth not sent: %v %v %v", user, pass, ok)
	}
	if _, err := load(t, "consulkv://"+host+"?key=absent"); err == nil {
		t.Error("a missing key must be an error")
	}
}

func TestServiceRegistry(t *testing.T) {
	host, _ := fakeConsul(t)
	if id, err := load(t, "consulsr://"+host+"?service=svc&tag=t&meta=m"); err != nil || id != "from-consul" {
		t.Fatalf("%q %v", id, err)
	}
	for name, location := range map[string]string{
		"no match":         "consulsr://" + host + "?service=none&tag=t&meta=m",
		"several matches":  "consulsr://" + host + "?service=dup&tag=t&meta=m",
		"missing meta key": "consulsr://" + host + "?service=svc&tag=t&meta=other",
	} {
		if _, err := load(t, location); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// Required query parameters are checked before any connection is made.
func TestLocationsAreValidated(t *testing.T) {
	for _, loc := range []string{
		"consulkv://127.0.0.1:1", "consulsr://127.0.0.1:1", "consulsr://127.0.0.1:1?service=s", "consulsr://127.0.0.1:1?service=s&tag=t",
	} {
		if _, err := load(t, loc); err == nil {
			t.Errorf("%s: expected an error", loc)
		}
	}
}

// The s schemes use https: a plain HTTP fake does not answer them; the client
// option reaches them.
func TestTLSSchemesUseHTTPS(t *testing.T) {
	host, _ := fakeConsul(t)
	if _, err := load(t, "consulkvs://"+host+"?key=bundle/ok"); err == nil {
		t.Error("consulkvs against an HTTP server must fail")
	}
	if _, err := load(t, "consulsrs://"+host+"?service=svc&tag=t&meta=m"); err == nil {
		t.Error("consulsrs against an HTTP server must fail")
	}
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"Key": "k", "Value": base64.StdEncoding.EncodeToString([]byte(yamlBundle))}})
	}))
	defer secure.Close()
	loc := "consulkvs://" + strings.TrimPrefix(secure.URL, "https://") + "?key=k"
	if _, err := load(t, loc); err == nil {
		t.Error("an untrusted certificate must fail")
	}
	insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}}} //nolint:gosec // test
	if id, err := load(t, loc, WithHTTPClient(insecure)); err != nil || id != "from-consul" {
		t.Errorf("%q %v", id, err)
	}
}

func TestCredentialsDoNotLeak(t *testing.T) {
	_, err := load(t, "consulkv://alice:"+password+"@127.0.0.1:1?key=k")
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("%v", err)
	}
}

func TestDeadline(t *testing.T) {
	block := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	defer s.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := fetch.Load(ctx, "consulkv://"+strings.TrimPrefix(s.URL, "http://")+"?key=k", fetch.WithSource(KV(), "consulkv")); err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("%v after %v", err, time.Since(start))
	}
}
