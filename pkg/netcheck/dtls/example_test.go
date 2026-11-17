package dtls_test

import (
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/dtls"
)

// Example registers the dtls runner: the core does not carry it, so that
// programs that do not check DTLS do not link its dependencies.
func Example() {
	_ = []netcheck.Option{netcheck.WithRunner(netcheck.DTLS, dtls.Runner())}
}
