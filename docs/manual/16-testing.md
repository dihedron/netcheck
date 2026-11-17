# Testing and release quality

## The layers

::: landscape

| Layer | What | Where |
| ------------------- | ------------------------------------------------------------------- | ---------------------------- |
| Unit | Marshalling, `Validate`, defaults, addresses, redaction, formats | `pkg/netcheck/*_test.go` |
| Per protocol, hermetic | Local listeners, `httptest`, an ephemeral CA, an in-process SSH server, a `pion/dtls` server, ICMP where the host allows it | `pkg/netcheck` |
| Guards | Import allow-list, no `init()`, no exported variables but `Err...`, no `os.Exit` or `panic` in `pkg/` | `guard_test.go` |
| Concurrency | `-race`, `goleak` in `TestMain` of every package, cancellation, a panicking and a hanging runner | `run_test.go` |
| Sources | File, HTTP and HTTPS hermetic; a Redis RESP fake and a Consul HTTP fake; real Redis and Consul in containers under `-tags integration` | `fetch/...` |
| Command | Exit codes, output formats, templates, defaults file, scheme layer | `cmd/netcheck` |
| Fuzz | `FuzzBundleUnmarshal`, `FuzzParse`, `FuzzSplitHostPort`: no panic, round trip | `go test -fuzz` |
| Documents | Every protocol, scheme, default and exit status is documented; the samples decode; every example named here exists; every `With...` option is mentioned | `docs/docs_test.go` |
| Stress and chaos | Thousands of checks against peers that misbehave at random | below |

:::

`go test -race ./...` needs no network and writes only to temporary directories.

## Stress and chaos

`make stress` runs `TestStress` (`-tags stress`) with the race detector for 20 seconds. Each round builds a bundle of 5,000 checks against local peers that randomly accept, refuse, say nothing, reset the connection, answer with a success, answer with an error or answer too late, and checks that:

1. every result is the status its peer was told to produce;
2. the run returns within `ceil(checks / concurrency) × (retries × timeout + wait)` plus a margin;
3. a run cancelled at a random moment returns every result, with `Skipped` only when the context ended, and promptly;
4. no goroutine outlives the test (`goleak`).

`NETCHECK_STRESS_DURATION` (seconds), `NETCHECK_STRESS_CHECKS` and `NETCHECK_STRESS_SEED` tune it; a failure prints its seed so the round can be replayed.

## Benchmark

`BenchmarkRunTCP` runs 1,000 TCP checks against a local listener. On a laptop with 16 hardware threads it took 68 ms at concurrency 1, 8.4 ms at 10 and 358 ms at 100: more workers than the listener's single accept loop can serve make it slower. The figures are a reference for the order of magnitude, not a guarantee.

## The overnight plan

Some verification takes hours or needs infrastructure; it is planned for an unattended session, not for the gate of a pull request.

1. **Soak.** `make soak` runs the stress test for 8 hours (`SOAK_SECONDS` changes it) with the race detector. Pass: no failure, no leak, and the process memory at the end close to the memory after the first rounds (watch it with `NETCHECK_MEM_PROFILE` on a command run in a loop).
2. **Flow test in the ligo lab.** The SDK's flow check against the real flows of a lab binding, with a firewall rule that blocks one port: the report must name the blocked flow, and only that one.
3. **Kerberos.** A KDC in a container, a client with a ticket, and a `sso: true` check against an HTTP server that negotiates: the real exchange, which the unit tests cannot cover.
4. **Packages.** Install the `.deb`, `.rpm` and `.apk` in containers, check the capability on the binary, run a bundle with an ICMP check as an ordinary user.
5. **Unprivileged ICMP pass path** on a host whose `ping_group_range` allows it.

## What is not covered

The real Kerberos exchange (item 3), the pass path of ICMP on hosts without the privilege, and the Windows build of the command's tests, which use `syscall.Kill`.
