package sso_test

import (
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/sso"
)

// Example gives the checks with sso: true a client that authenticates with
// the local Kerberos or NTLM identity. Pass the same client to
// fetch.WithHTTPClient to load bundles from a server that needs it.
func Example() {
	_ = []netcheck.Option{netcheck.WithSSOClient(sso.Client())}
}
