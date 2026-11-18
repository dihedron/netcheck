//go:build lab

package acceptance

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// LB-07: the soak. For hours, the installed command runs a bundle of 200
// checks against the real servers while a seeded scheduler injects one fault
// at a time (or none); every report must match the truth table of its fault.
// In the same time a race-enabled stress binary runs on nc-client, and its
// memory is sampled; the judge at the end fails on any mismatch, on a stress
// failure, and on memory that keeps growing. Start it with lab.sh soak.

// scenario is one fault and the checks it must make fail.
type scenario struct {
	name    string
	apply   func(t *testing.T) func() // returns the undo
	failing []string
}

func nftFault(rule string) func(t *testing.T) func() {
	return func(t *testing.T) func() {
		if r := run(t, "nc-fw", "sh", "-c", "nft add rule inet lab faults "+rule); r.Code != 0 {
			t.Fatalf("fault %q: %s", rule, r.Stderr)
		}
		return func() { run(t, "nc-fw", "nft", "flush", "chain", "inet", "lab", "faults") }
	}
}

func stopService(vm string, units ...string) func(t *testing.T) func() {
	return func(t *testing.T) func() {
		run(t, vm, append([]string{"systemctl", "stop"}, units...)...)
		return func() { run(t, vm, append([]string{"systemctl", "start"}, units...)...) }
	}
}

func netem(delay string) func(t *testing.T) func() {
	return func(t *testing.T) func() {
		run(t, "nc-fw", "tc", "qdisc", "replace", "dev", "enp6s0", "root", "netem", "delay", delay)
		return func() { run(t, "nc-fw", "tc", "qdisc", "del", "dev", "enp6s0", "root") }
	}
}

func scenarios() []scenario {
	s := serversIP()
	return []scenario{
		{"calm", func(*testing.T) func() { return func() {} }, nil},
		{"calm", func(*testing.T) func() { return func() {} }, nil},
		{"calm", func(*testing.T) func() { return func() {} }, nil},
		{"drop web ports", nftFault("ip daddr " + s + " tcp dport { 80, 8443 } counter drop"), []string{"tls", "http", "https"}},
		{"reset 9001", nftFault("ip daddr " + s + " tcp dport 9001 counter reject with tcp reset"), []string{"tcp"}},
		{"drop icmp", nftFault("ip daddr " + s + " icmp type echo-request counter drop"), []string{"icmp"}},
		{"drop dtls", nftFault("ip daddr " + s + " udp dport 4433 counter drop"), []string{"dtls"}},
		{"stop nginx", stopService("nc-servers", "nginx"), []string{"tls", "http", "https"}},
		{"stop sshd", stopService("nc-servers", "ssh.socket", "ssh"), []string{"ssh"}},
		{"stop tcp listener", stopService("nc-servers", "lab-tcp-open"), []string{"tcp"}},
		{"delay 4.5 s", netem("4500ms"), []string{"tcp", "tls", "http", "https", "ssh", "icmp", "dtls"}},
	}
}

// soakBundle is the faults bundle, 25 times: 200 checks named kind#n.
func soakBundle() string {
	var b strings.Builder
	b.WriteString("id: soak\nretries: 1\ntimeout: 4s\nwait: 10ms\nconcurrency: 64\nchecks:\n")
	kinds := []struct{ name, proto, addr string }{
		{"tcp", "tcp", "10.78.0.10:9001"}, {"tls", "tls", "nc-servers.lab:8443"}, {"http", "http", "nc-servers.lab/ok"},
		{"https", "https", "nc-servers.lab:8443/ok"}, {"udp", "udp", "10.78.0.10:9101"}, {"ssh", "ssh", "10.78.0.10"},
		{"icmp", "icmp", "10.78.0.10"}, {"dtls", "dtls", "nc-servers.lab:4433"},
	}
	for i := 0; i < 25; i++ {
		for _, k := range kinds {
			fmt.Fprintf(&b, "  - {name: \"%s#%d\", protocol: %s, address: \"%s\"}\n", k.name, i, k.proto, k.addr)
		}
	}
	return b.String()
}

type cycle struct {
	N         int      `json:"n"`
	Scenario  string   `json:"scenario"`
	Seconds   float64  `json:"seconds"`
	Mismatch  []string `json:"mismatch,omitempty"`
	StartedAt string   `json:"started_at"`
}

func TestSoak(t *testing.T) {
	secondsEnv := os.Getenv("NETCHECK_SOAK_SECONDS")
	if secondsEnv == "" {
		t.Skip("the soak is started by lab.sh soak (NETCHECK_SOAK_SECONDS)")
	}
	seconds, err := strconv.Atoi(secondsEnv)
	if err != nil || seconds <= 0 {
		t.Fatalf("NETCHECK_SOAK_SECONDS=%q", secondsEnv)
	}
	seed := time.Now().UnixNano()
	if v := os.Getenv("NETCHECK_SOAK_SEED"); v != "" {
		seed, _ = strconv.ParseInt(v, 10, 64)
	}
	out := os.Getenv("NETCHECK_LAB_STATE")
	if out == "" {
		out = "build/lab"
	}
	out = filepath.Join(out, "soak")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Logf("soak of %d s, seed %d, results in %s", seconds, seed, out)
	rng := rand.New(rand.NewSource(seed)) //nolint:gosec // a test: the seed is printed to replay a run

	// the stress binary, long-lived on the client
	// two of them: the race-enabled one finds races, the plain one is the one
	// whose memory is judged (the race detector's own memory grows with the run
	// and is no measure of the library)
	run(t, "nc-client", "sh", "-c", "pkill -f \"[r]ace-slices\"; pkill -f [r]ace-slices; pkill -f \"[n]etcheck.stress\"; rm -f /root/stress-*.log; true")
	startStress := func(binary, name string, checks int) string {
		run(t, "nc-client", "sh", "-c", fmt.Sprintf("NETCHECK_STRESS_DURATION=%d NETCHECK_STRESS_CHECKS=%d nohup nice -n 10 /root/%s -test.run 'TestStress$' -test.v -test.timeout 0 >/root/stress-%s.log 2>&1 & echo $! >/root/stress-%s.pid", seconds, checks, binary, name, name))
		return strings.TrimSpace(run(t, "nc-client", "cat", "/root/stress-"+name+".pid").Stdout)
	}
	// the race binary's memory grows by gigabytes in minutes (race detector
	// shadow memory, not the library: the plain binary stays flat), which starves
	// the client; it runs in 4-minute slices, each a fresh process
	runIn(t, "nc-client", fmt.Sprintf(`end=$(( $(date +%%s) + %d ))
while [ "$(date +%%s)" -lt "$end" ]; do
  d=$(( end - $(date +%%s) )); [ "$d" -gt 240 ] && d=240
  NETCHECK_STRESS_DURATION=$d NETCHECK_STRESS_CHECKS=2000 nice -n 10 /root/netcheck.stress.race -test.run 'TestStress$' -test.v -test.timeout 0 || exit 1
done
`, seconds), "sh", "-c", "cat >/root/race-slices.sh")
	run(t, "nc-client", "sh", "-c", "nohup sh /root/race-slices.sh >/root/stress-race.log 2>&1 & echo $! >/root/stress-race.pid")
	racePID := strings.TrimSpace(run(t, "nc-client", "cat", "/root/stress-race.pid").Stdout)
	pid := startStress("netcheck.stress.plain", "plain", 5000)

	runIn(t, "nc-client", soakBundle(), "sh", "-c", "cat >/tmp/soak.yaml; chmod 644 /tmp/soak.yaml")
	cycles, err := os.Create(filepath.Join(out, "cycles.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer cycles.Close()
	samples, err := os.Create(filepath.Join(out, "samples.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer samples.Close()
	fmt.Fprintln(samples, "seconds,rss_kib,fds,race_rss_kib")

	start := time.Now()
	deadline := start.Add(time.Duration(seconds) * time.Second)
	nextSample := start
	var mismatches []string
	all := scenarios()
	for n := 0; time.Now().Before(deadline); n++ {
		if !time.Now().Before(nextSample) {
			rss := strings.TrimSpace(run(t, "nc-client", "sh", "-c", "ps -o rss= -p "+pid+" || true").Stdout)
			fds := strings.TrimSpace(run(t, "nc-client", "sh", "-c", "ls /proc/"+pid+"/fd 2>/dev/null | wc -l").Stdout)
			raceRSS := strings.TrimSpace(run(t, "nc-client", "sh", "-c", "ps -o rss= --ppid "+racePID+" || true").Stdout)
			fmt.Fprintf(samples, "%d,%s,%s,%s\n", int(time.Since(start).Seconds()), rss, fds, raceRSS)
			nextSample = nextSample.Add(time.Minute)
		}
		sc := all[rng.Intn(len(all))]
		began := time.Now()
		undo := sc.apply(t)
		time.Sleep(1500 * time.Millisecond) // let the fault settle
		reports, code := runSoakBundle(t)
		undo()
		time.Sleep(500 * time.Millisecond) // and the repair
		c := cycle{N: n, Scenario: sc.name, Seconds: time.Since(began).Seconds(), StartedAt: began.Format(time.RFC3339)}
		c.Mismatch = judgeCycle(reports, code, sc.failing)
		for _, m := range c.Mismatch {
			mismatches = append(mismatches, fmt.Sprintf("cycle %d (%s): %s", n, sc.name, m))
		}
		line, _ := json.Marshal(c)
		fmt.Fprintln(cycles, string(line))
	}

	// the stress binaries end with the duration; wait for them
	for _, name := range []string{"plain", "race"} {
		stressPID := map[string]string{"plain": pid, "race": racePID}[name]
		for i := 0; i < 120; i++ {
			if r := run(t, "nc-client", "sh", "-c", "kill -0 "+stressPID+" 2>/dev/null"); r.Code != 0 {
				break
			}
			time.Sleep(5 * time.Second)
		}
		log := run(t, "nc-client", "cat", "/root/stress-"+name+".log").Stdout
		_ = os.WriteFile(filepath.Join(out, "stress-"+name+".log"), []byte(log), 0o644) //nolint:gosec // a log
		if !strings.Contains(log, "--- PASS: TestStress") || strings.Contains(log, "FAIL") || strings.Contains(log, "DATA RACE") {
			t.Errorf("the %s stress binary did not pass:\n%s", name, tail(log, 40))
		}
	}
	for _, m := range mismatches {
		t.Error(m)
	}
	if grew, why := memoryGrew(filepath.Join(out, "samples.csv")); grew {
		t.Errorf("memory: %s", why)
	}
	t.Logf("soak done: %d mismatches, see %s", len(mismatches), out)
}

func tail(s string, lines int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}

func runSoakBundle(t *testing.T) ([]*netcheck.Report, int) {
	t.Helper()
	var reports []*netcheck.Report
	var r result
	for attempt := 0; attempt < 3; attempt++ {
		r = asProbe(t, "netcheck", "--format=json", "/tmp/soak.yaml")
		if r.Code <= 2 { // anything else is lxc exec failing, not the command
			break
		}
		time.Sleep(2 * time.Second)
	}
	if r.Stdout != "" {
		_ = json.Unmarshal([]byte(r.Stdout), &reports)
	}
	return reports, r.Code
}

// judgeCycle compares a report to the truth table: exactly the failing kinds failed.
func judgeCycle(reports []*netcheck.Report, code int, failing []string) []string {
	if len(reports) != 1 || len(reports[0].Results) != 200 {
		return []string{fmt.Sprintf("no usable report (exit %d)", code)}
	}
	want := map[string]bool{}
	for _, k := range failing {
		want[k] = true
	}
	wrong := map[string]int{}
	for _, r := range reports[0].Results {
		kind, _, _ := strings.Cut(r.Check.Name, "#")
		expected := netcheck.Passed
		if want[kind] {
			expected = netcheck.Failed
		}
		if r.Status != expected {
			wrong[kind+" is "+string(r.Status)+", want "+string(expected)]++
			if r.Err != nil && !want[kind] && wrong["e.g. "+kind] == 0 {
				wrong["e.g. "+kind] = 1
				wrong["  e.g. "+r.Check.Name+" ("+r.Latency.String()+", "+strconv.Itoa(r.Attempts)+" attempts): "+r.Err.Error()] = 1
			}
		}
	}
	var out []string
	for k, n := range wrong {
		out = append(out, fmt.Sprintf("%s (%d checks)", k, n))
	}
	sort.Strings(out)
	if wantCode := map[bool]int{true: 2, false: 0}[len(failing) > 0]; code != wantCode {
		out = append(out, fmt.Sprintf("exit %d, want %d", code, wantCode))
	}
	return out
}

// memoryGrew reads the samples (seconds,rss_kib,fds) and says whether the
// stress binary's memory or descriptors kept growing: the largest value of the
// last quarter must not exceed 1.5 times the median of the window from 10% to
// 35% of the run plus 64 MiB (for memory) or plus 32 (for descriptors).
func memoryGrew(path string) (bool, string) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return true, err.Error()
	}
	var rss, fds []int
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		f := strings.Split(line, ",")
		if len(f) != 4 {
			continue
		}
		r, err1 := strconv.Atoi(f[1])
		d, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			continue
		}
		rss, fds = append(rss, r), append(fds, d)
	}
	if len(rss) < 8 {
		return false, "" // too short to judge
	}
	judge := func(series []int, slack int, unit string) (bool, string) {
		n := len(series)
		window := append([]int(nil), series[n/10:n*35/100+1]...)
		sort.Ints(window)
		median := window[len(window)/2]
		peak := 0
		for _, v := range series[n*3/4:] {
			if v > peak {
				peak = v
			}
		}
		if limit := median*3/2 + slack; peak > limit {
			return true, fmt.Sprintf("%s peaked at %d in the last quarter, over %d (1.5 x the early median %d + %d)", unit, peak, limit, median, slack)
		}
		return false, ""
	}
	if grew, why := judge(rss, 64*1024, "RSS (KiB)"); grew {
		return true, why
	}
	return judge(fds, 32, "descriptors")
}
