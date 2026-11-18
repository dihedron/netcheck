package netcheck_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

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

func ExampleRunCheck() {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer func() { _ = l.Close() }()

	result := netcheck.RunCheck(context.Background(), netcheck.Check{
		Name:     "listening",
		Address:  l.Addr().String(),
		Protocol: netcheck.TCP,
	})
	fmt.Println(result.Status, result.Attempts, result.Err)
	// Output: passed 1 <nil>
}

func ExampleBundle_Validate() {
	bundle := &netcheck.Bundle{
		Checks: []netcheck.Check{
			{Address: "db.example.com:5432", Protocol: "gopher"},
			{Address: "", Protocol: netcheck.TCP},
		},
	}
	// every defect is reported, not only the first
	err := bundle.Validate()
	fmt.Println(err != nil)
	// Output: true
}

func ExampleWithRunner() {
	// a runner replaces the built-in one of a protocol: here, a fake that
	// needs no network
	fake := netcheck.RunnerFunc(func(ctx context.Context, c netcheck.Check, d netcheck.Defaults) (netcheck.Detail, error) {
		return netcheck.Detail{Remote: "192.0.2.1:80"}, nil
	})
	result := netcheck.RunCheck(context.Background(),
		netcheck.Check{Address: "service.example.com:80", Protocol: netcheck.TCP},
		netcheck.WithRunner(netcheck.TCP, fake))
	fmt.Println(result.Status, result.Detail.Remote)
	// Output: passed 192.0.2.1:80
}

func ExampleWithDeniedNetworks() {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer func() { _ = l.Close() }()

	// the loopback network is denied: the check fails before connecting
	result := netcheck.RunCheck(context.Background(),
		netcheck.Check{Address: l.Addr().String(), Protocol: netcheck.TCP, Retries: 1},
		netcheck.WithDeniedNetworks(netip.MustParsePrefix("127.0.0.0/8")))
	fmt.Println(result.Status, errors.Is(result.Err, netcheck.ErrNetworkNotAllowed))
	// Output: failed true
}

func ExampleReport_Failed() {
	report := &netcheck.Report{Results: []netcheck.Result{
		{Status: netcheck.Passed},
		{Status: netcheck.Failed},
		{Status: netcheck.Skipped},
	}}
	// a skipped check is not a failure, but it is not a pass either
	fmt.Println(report.Failed(), report.OK())
	// Output: 1 false
}
