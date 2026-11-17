package netcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

func FuzzBundleUnmarshal(f *testing.F) {
	f.Add([]byte(`{"id":"x","timeout":"2s","checks":[{"address":"h:1","protocol":"tcp"}]}`), true)
	f.Add([]byte("id: x\nchecks:\n  - address: h:1\n"), false)
	f.Fuzz(func(t *testing.T, data []byte, asJSON bool) {
		var b Bundle
		var err error
		if asJSON {
			err = json.Unmarshal(data, &b)
		} else {
			err = yaml.Unmarshal(data, &b)
		}
		if err != nil {
			return
		}
		_ = b.Validate() // must not panic, whatever was read
		out, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("a bundle that was read must be written: %v", err)
		}
		var again Bundle
		if err := json.Unmarshal(out, &again); err != nil {
			t.Fatalf("round trip: %v", err)
		}
	})
}

// BenchmarkRunTCP is the reference for the guide: 1,000 checks against a local listener.
func BenchmarkRunTCP(b *testing.B) {
	addr := listenerB(b)
	for _, n := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("concurrency=%d", n), func(b *testing.B) {
			bundle := &Bundle{Concurrency: n, Retries: 1, Checks: make([]Check, 1000)}
			for i := range bundle.Checks {
				bundle.Checks[i] = Check{Address: addr, Protocol: TCP}
			}
			for b.Loop() {
				if rep, err := Run(context.Background(), bundle); err != nil || !rep.OK() {
					b.Fatalf("%v %v", err, rep.Failed())
				}
			}
		})
	}
}
