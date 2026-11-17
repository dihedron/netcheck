# Using the library

```bash
$> go get github.com/dihedron/netcheck/v2@latest
```

```go
import "github.com/dihedron/netcheck/v2/pkg/netcheck"
```

## Running a bundle

`Run` executes every check of a `Bundle`, at most `Concurrency` at a time, and returns a `Report` with one `Result` per check, in the order of `Bundle.Checks`. It does not modify the bundle.

```go
bundle := &netcheck.Bundle{
    ID:      "payments",
    Retries: 2,
    Checks: []netcheck.Check{
        {Name: "database", Address: "db.example.com:5432", Protocol: netcheck.TCP},
        {Name: "api", Address: "api.example.com/health", Protocol: netcheck.HTTPS},
    },
}
report, err := netcheck.Run(ctx, bundle)
```

`Run` returns an error and no report when the bundle is invalid (`Bundle.Validate` reports every defect at once, joined). When the context ends early it returns the report, with the unfinished checks `Skipped`, and the context's error. The runnable `ExampleBundle_Validate` shows the validation and `Example` a complete run.

`RunCheck` runs one check with the same semantics and returns its `Result`; see `ExampleRunCheck`. A program that expands its own flows into checks, as the ligo SDK does, builds a `[]Check` and calls `Run`.

## Options

| Option | Effect |
| ------------------------------ | ---------------------------------------------------------- |
| `WithDefaults(d)` | Replaces the compiled-in `Defaults` (timeout, retries, wait, concurrency, UDP grace, ping) |
| `WithLimits(l)` | Replaces the bounds `Validate` enforces (`DefaultLimits`: 10 attempts, 5 min, 256 workers, 1,000 checks) |
| `WithLogger(l)` | Where the library logs; by default nowhere |
| `WithRunner(p, r)` | Registers the `Runner` of a protocol, replacing a built-in one; see `ExampleWithRunner` |
| `WithHTTPClient(h)` | The client of http and https checks; never modified, copied when a check needs its own transport |
| `WithSSOClient(h)` | The client of checks with `sso: true` (chapter 15) |
| `WithAllowedNetworks(p...)` | Only connect inside these networks |
| `WithDeniedNetworks(p...)` | Never connect inside these networks; wins over the allowed ones; see `ExampleWithDeniedNetworks` |

A check that is stopped by the networks limits fails with an error that wraps `ErrNetworkNotAllowed`.

## Results

```go
for _, r := range report.Results {
    switch r.Status {
    case netcheck.Passed:
    case netcheck.Failed:
        log.Printf("%s: %v (after %d attempts)", r.Check.Name, r.Err, r.Attempts)
    case netcheck.Skipped:
    }
}
```

`Result.Err` is the error of the last attempt and supports `errors.Is` and `errors.As` (`context.DeadlineExceeded`, `x509.CertificateInvalidError`, `ErrNetworkNotAllowed`...). `Result.Detail` holds the evidence: the address connected to, the HTTP status, the ICMP packet counts, the `TLSFacts` of a certificate (kept even when verification failed) and the SSH banner and host-key fingerprint. `Report.OK` is true when every check passed (a skipped check is not a pass) and `Report.Failed` counts the failures; see `ExampleReport_Failed`.

`Report` and `Result` marshal to the JSON and YAML shape of chapter 7, durations and errors as strings, and read back.

## TLS options

TLS options are set through the API only, never read from a bundle file: `Check.TLS` takes a `TLSOptions` with a `ServerName` to verify, a `RootCAs` pool, or `InsecureSkipVerify`, in which case the result records `Verified: false`. A flow checked against a workload's own trust bundle sets `RootCAs` and `ServerName` and leaves the rest alone. `Check.AllowDowngrade` lets an https check follow a redirect to http; it is off by default.

## Concurrency and cancellation

The library keeps no state: any number of goroutines can call `Run` at the same time with different options. Pass a context with a deadline to bound the whole run; every attempt also has its own deadline from the check's timeout. A `Runner` of your own must return when its context ends.

## Guarantees and limits

The guarantee of each protocol is in chapter 11. The library never reads a file, never writes to the terminal and never exits; it logs only through the logger it is given, with credentials removed from addresses.
