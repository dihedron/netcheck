package netcheck

import (
	"strings"
	"testing"
	"time"
)

// NC-15: every problem is reported at once.
func TestValidateReportsEverything(t *testing.T) {
	b := &Bundle{
		Retries: 99, Timeout: Duration(time.Hour), Concurrency: 1000,
		Checks: []Check{
			{Address: "a:1"},                     // no protocol
			{Address: "", Protocol: TCP},         // no address
			{Address: "a:1", Protocol: "gopher"}, // unknown protocol
			{Address: "no-port", Protocol: TCP},  // tcp needs a port
			{Address: "a:1", Protocol: TCP, Retries: 11},
			{Address: "a:1", Protocol: TCP, Timeout: Duration(10 * time.Minute)},
		},
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("want errors")
	}
	for _, want := range []string{"retries", "timeout", "concurrency", "protocol", "address", "gopher", "port", "check 0", "check 5"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestValidateAcceptsGoodBundle(t *testing.T) {
	b := &Bundle{Checks: []Check{
		{Address: "a:1", Protocol: TCP}, {Address: "h", Protocol: ICMP}, {Address: "h", Protocol: SSH},
		{Address: "[2001:db8::1]:443", Protocol: TLS}, {Address: "h/p?q=1", Protocol: HTTPS},
	}}
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMaxChecks(t *testing.T) {
	b := &Bundle{Checks: make([]Check, 3)}
	for i := range b.Checks {
		b.Checks[i] = Check{Address: "a:1", Protocol: TCP}
	}
	l := DefaultLimits()
	l.MaxChecks = 2
	if err := b.validate(l); err == nil || !strings.Contains(err.Error(), "checks") {
		t.Fatalf("want too many checks, got %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}
}
