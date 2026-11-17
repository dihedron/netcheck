package checks

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestTimeoutRoundTrip(t *testing.T) {
	in := Timeout(1500 * time.Millisecond)
	if in.String() != "1.5s" {
		t.Errorf("String: %s", in)
	}
	data, err := json.Marshal(in)
	if err != nil || string(data) != `"1.5s"` {
		t.Fatalf("JSON: %s, %v", data, err)
	}
	var j Timeout
	if err := json.Unmarshal(data, &j); err != nil || j != in {
		t.Errorf("JSON round trip: %v, %v", j, err)
	}
	out, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var y Timeout
	if err := yaml.Unmarshal(out, &y); err != nil || y != in {
		t.Errorf("YAML round trip: %v, %v", y, err)
	}
	text, _ := in.MarshalText()
	var x Timeout
	if err := x.UnmarshalText(text); err != nil || x != in {
		t.Errorf("text round trip: %v, %v", x, err)
	}
	for _, bad := range []string{`"soon"`, `5`} {
		if err := json.Unmarshal([]byte(bad), &x); err == nil {
			t.Errorf("JSON %s: expected an error", bad)
		}
	}
	if err := yaml.Unmarshal([]byte("soon"), &x); err == nil {
		t.Error("YAML: expected an error")
	}
	if err := x.UnmarshalText([]byte("soon")); err == nil {
		t.Error("text: expected an error")
	}
}

func TestProtocolRoundTrip(t *testing.T) {
	for _, p := range []Protocol{TCP, UDP, ICMP, TLS, DTLS, SSH, HTTP, HTTPS} {
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var j Protocol
		if err := json.Unmarshal(data, &j); err != nil || j != p {
			t.Errorf("JSON %s: %v, %v", p, j, err)
		}
		out, _ := yaml.Marshal(p)
		var y Protocol
		if err := yaml.Unmarshal(out, &y); err != nil || y != p {
			t.Errorf("YAML %s: %v, %v", p, y, err)
		}
		text, _ := p.MarshalText()
		var x Protocol
		if err := x.UnmarshalText(text); err != nil || x != p {
			t.Errorf("text %s: %v, %v", p, x, err)
		}
	}
	var p Protocol
	if err := p.FromString("gopher"); err == nil {
		t.Error("an unknown protocol must be refused")
	}
	if err := json.Unmarshal([]byte(`"gopher"`), &p); err == nil {
		t.Error("JSON: an unknown protocol must be refused")
	}
	if err := json.Unmarshal([]byte(`7`), &p); err == nil {
		t.Error("JSON: a number is not a protocol")
	}
}

func TestResult(t *testing.T) {
	ok := Result{}
	if ok.IsError() || ok.String() != "success" {
		t.Errorf("success: %v %s", ok.IsError(), ok)
	}
	failed := Result{err: errors.New("boom")}
	if !failed.IsError() || failed.String() != "boom" {
		t.Errorf("failure: %v %s", failed.IsError(), failed)
	}
	data, _ := json.Marshal(failed)
	if string(data) != `"boom"` {
		t.Errorf("JSON: %s", data)
	}
	out, _ := yaml.Marshal(ok)
	if string(out) != "success\n" {
		t.Errorf("YAML: %q", out)
	}
}
