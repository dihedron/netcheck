package netcheck

import "testing"

func TestSplitHostPort(t *testing.T) {
	for _, tc := range []struct {
		in, def, host, port string
		ok                  bool
	}{
		{"h:80", "", "h", "80", true},
		{"h", "22", "h", "22", true},
		{"h", "", "", "", false},
		{"[2001:db8::1]:443", "", "2001:db8::1", "443", true},
		{"[2001:db8::1]", "443", "2001:db8::1", "443", true},
		{"2001:db8::1", "443", "2001:db8::1", "443", true}, // a bare IPv6 literal
		{"", "80", "", "", false},
		{":80", "", "", "", false},
		{"h:", "22", "", "", false},
	} {
		host, port, err := splitHostPort(tc.in, tc.def)
		if (err == nil) != tc.ok || host != tc.host || port != tc.port {
			t.Errorf("splitHostPort(%q,%q) = %q %q %v", tc.in, tc.def, host, port, err)
		}
	}
}

// NC-13: default ports for ssh, http and https.
func TestTargetDefaultPorts(t *testing.T) {
	for _, tc := range []struct {
		p    Protocol
		in   string
		want string
	}{
		{SSH, "h", "h:22"}, {SSH, "h:2222", "h:2222"},
		{HTTP, "h/x/y", "h:80"}, {HTTPS, "h", "h:443"}, {HTTPS, "[::1]/x", "[::1]:443"}, {HTTPS, "h:8443/x", "h:8443"},
		{TCP, "h:9", "h:9"}, {TLS, "[::1]:9", "[::1]:9"},
	} {
		got, err := dialTarget(tc.p, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("dialTarget(%s,%q) = %q %v, want %q", tc.p, tc.in, got, err, tc.want)
		}
	}
	if _, err := dialTarget(TCP, "h"); err == nil {
		t.Error("tcp needs a port")
	}
}

func TestRedact(t *testing.T) {
	for in, want := range map[string]string{
		"user:s3cret@host:80/x": "host:80/x",
		"host:80":               "host:80",
		"a@b@host":              "host",
	} {
		if got := redact(in); got != want {
			t.Errorf("redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func FuzzSplitHostPort(f *testing.F) {
	for _, s := range []string{"h:1", "[::1]:2", "", "[", "a:b:c", "h:99999"} {
		f.Add(s, "22")
	}
	f.Fuzz(func(t *testing.T, in, def string) {
		host, port, err := splitHostPort(in, def)
		if err == nil && (host == "" || port == "") {
			t.Fatalf("no error but %q %q", host, port)
		}
	})
}
