package netcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDurationRoundTrip(t *testing.T) {
	in := Check{Address: "a:1", Protocol: TCP, Timeout: Duration(2 * time.Second), Wait: Duration(150 * time.Millisecond)}
	data, err := json.Marshal(in)
	if err != nil || !strings.Contains(string(data), `"timeout":"2s"`) || !strings.Contains(string(data), `"wait":"150ms"`) {
		t.Fatalf("json: %s %v", data, err)
	}
	var out Check
	if err := json.Unmarshal(data, &out); err != nil || out != in {
		t.Fatalf("json round trip: %+v %v", out, err)
	}
	data, err = yaml.Marshal(in)
	if err != nil || !strings.Contains(string(data), "timeout: 2s") {
		t.Fatalf("yaml: %s %v", data, err)
	}
	out = Check{}
	if err := yaml.Unmarshal(data, &out); err != nil || out != in {
		t.Fatalf("yaml round trip: %+v %v", out, err)
	}
}

func TestDurationRejectsGarbage(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"soon"`), &d); err == nil {
		t.Error("json: want an error")
	}
	if err := json.Unmarshal([]byte(`5`), &d); err == nil {
		t.Error("json: a bare number is not a duration")
	}
	if err := yaml.Unmarshal([]byte(`soon`), &d); err == nil {
		t.Error("yaml: want an error")
	}
}

// NC-14: the sample bundles load unchanged; a check without protocol read
// from a file is a tcp check, as it always was.
func TestSampleBundlesLoad(t *testing.T) {
	for _, name := range []string{"netcheck.yaml", "netcheck.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "test", "fixtures", name))
		if err != nil {
			t.Fatal(err)
		}
		var b Bundle
		if strings.HasSuffix(name, ".json") {
			err = json.Unmarshal(data, &b)
		} else {
			err = yaml.Unmarshal(data, &b)
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(b.Checks) == 0 || b.ID == "" {
			t.Fatalf("%s: empty bundle %+v", name, b)
		}
		if err := b.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if b.Checks[0].Protocol != TCP {
			t.Errorf("%s: a check without protocol must be tcp, got %q", name, b.Checks[0].Protocol)
		}
	}
}

func TestBundleRoundTrip(t *testing.T) {
	in := Bundle{ID: "x", Description: "d", Timeout: Duration(time.Second), Retries: 2, Concurrency: 3,
		Checks: []Check{{Name: "n", Address: "h:443", Protocol: TLS, SSO: false}}}
	data, _ := json.Marshal(in)
	var out Bundle
	if err := json.Unmarshal(data, &out); err != nil || out.ID != "x" || out.Checks[0] != in.Checks[0] {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestUnknownProtocolIsRejectedByValidate(t *testing.T) {
	var b Bundle
	if err := json.Unmarshal([]byte(`{"id":"x","checks":[{"address":"a:1","protocol":"gopher"}]}`), &b); err != nil {
		t.Fatal(err)
	}
	if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "gopher") {
		t.Fatalf("want an error naming the protocol, got %v", err)
	}
}

func TestDefaultDefaults(t *testing.T) {
	d := DefaultDefaults()
	if d.Timeout != Duration(20*time.Second) || d.Retries != 3 || d.Wait != Duration(time.Second) || d.Concurrency != 10 ||
		d.UDPGrace != Duration(200*time.Millisecond) || d.Ping.Count != 10 || d.Ping.Size != 64 || d.Ping.Privileged {
		t.Errorf("unexpected defaults %+v", d)
	}
}
