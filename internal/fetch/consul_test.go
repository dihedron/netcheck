package fetch

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/internal/format"
)

const yamlBundle = "---\nid: from-consul\nchecks:\n  - address: localhost:80\n"

// fakeConsul answers the two Consul HTTP APIs the fetchers use: the key/value
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

func TestFromConsulKV(t *testing.T) {
	host, last := fakeConsul(t)
	data, f, err := FromConsulKV("consulkv://" + host + "?key=bundle/ok&dc=eu1")
	if err != nil || string(data) != yamlBundle || f != format.YAML {
		t.Fatalf("got %q, %v, %v", data, f, err)
	}
	if last().URL.Query().Get("dc") != "eu1" {
		t.Errorf("the datacentre was not passed: %s", last().URL)
	}
	// an ACL token (empty user) and basic credentials
	if _, _, err := FromConsulKV("consulkv://:sekret@" + host + "?key=bundle/ok"); err != nil || last().Header.Get("X-Consul-Token") != "sekret" {
		t.Errorf("token: %v, header %q", err, last().Header.Get("X-Consul-Token"))
	}
	if _, _, err := FromConsulKV("consulkv://alice:pw@" + host + "?key=bundle/ok"); err != nil {
		t.Fatal(err)
	}
	if user, pass, ok := last().BasicAuth(); !ok || user != "alice" || pass != "pw" {
		t.Errorf("basic auth not sent: %v %v %v", user, pass, ok)
	}
	if _, _, err := FromConsulKV("consulkv://" + host + "?key=absent"); err == nil {
		t.Error("a missing key must be an error")
	}
}

func TestFromConsulSR(t *testing.T) {
	host, _ := fakeConsul(t)
	data, f, err := FromConsulSR("consulsr://" + host + "?service=svc&tag=t&meta=m")
	if err != nil || string(data) != yamlBundle || f != format.YAML {
		t.Fatalf("got %q, %v, %v", data, f, err)
	}
	for name, location := range map[string]string{
		"no match":         "consulsr://" + host + "?service=none&tag=t&meta=m",
		"several matches":  "consulsr://" + host + "?service=dup&tag=t&meta=m",
		"missing meta key": "consulsr://" + host + "?service=svc&tag=t&meta=other",
	} {
		if _, _, err := FromConsulSR(location); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// The TLS schemes use https: a plain HTTP fake does not answer them.
func TestConsulTLSSchemesUseHTTPS(t *testing.T) {
	host, _ := fakeConsul(t)
	if _, _, err := FromConsulKV("consulkvs-://" + host + "?key=bundle/ok"); err == nil {
		t.Error("consulkvs-:// against an HTTP server must fail")
	}
	if _, _, err := FromConsulSR("consulsrs://" + host + "?service=svc&tag=t&meta=m"); err == nil {
		t.Error("consulsrs:// against an HTTP server must fail")
	}
}
