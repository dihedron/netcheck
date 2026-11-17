package netcheck_test

import (
	"context"
	"fmt"
	"net"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

func Example() {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer func() { _ = l.Close() }()
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	gone := closed.Addr().String()
	_ = closed.Close()

	bundle := &netcheck.Bundle{
		ID:      "example",
		Retries: 1,
		Checks: []netcheck.Check{
			{Name: "listening", Address: l.Addr().String(), Protocol: netcheck.TCP},
			{Name: "closed", Address: gone, Protocol: netcheck.TCP},
		},
	}
	report, err := netcheck.Run(context.Background(), bundle)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, r := range report.Results {
		fmt.Println(r.Check.Name, r.Status)
	}
	fmt.Println("failed:", report.Failed())
	// Output:
	// listening passed
	// closed failed
	// failed: 1
}
