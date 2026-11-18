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

## The lab

`test/lab/lab.sh` builds a lab of three LXD virtual machines on a host with KVM, in a project, two networks and a storage pool of their own (all tagged `user.netcheck-lab=yes`, so the lab never touches anything else), and runs the **installed package** against **real servers**:

| VM | Role |
| ------------ | ---------------------------------------------------------------------------- |
| `nc-servers` | nginx (HTTP, and HTTPS on four ports with a good, an expired, a wrong-name and an untrusted certificate), OpenSSH, TCP and UDP listeners, a DTLS 1.2 server, Redis (plain and TLS), Consul (key/value, service registry, HTTPS), a Kerberos KDC with Apache `mod_auth_gssapi` for SSO |
| `nc-client` | Ubuntu without sudo: the `.deb` from `make snapshot` is installed with `dpkg -i` and run as an ordinary user |
| `nc-fw` | The router between the client's network and the servers' network, with an nftables fault chain and `tc netem`, so that every packet crosses it and the tests can drop, reject or delay it |

| Command | Does |
| ----------------------------- | ---------------------------------------------------------------------- |
| `make lab-up` | Builds or completes the lab (idempotent); certificates, the Kerberos keys and the passwords are generated on the first run under `build/lab/` and never committed |
| `make lab-deploy` | Installs the newest `dist/netcheck_*_linux_amd64.deb` (`make snapshot` builds it) on the client |
| `make lab-test` | Runs the scenarios below; about 2 minutes |
| `make lab-soak` | The soak (below); refuses to start between 08:00 and 20:00 unless told `--now` |
| `make lab-down` | Destroys the lab after asking |

The scenarios (`test/lab/acceptance`, build tag `lab`, driving the installed command through `lxc exec` and reading its JSON output):

| ID | Scenario |
| ------ | --------------------------------------------------------------------------- |
| LB-01 | The package sets `cap_net_raw`, so an ordinary user's ICMP check passes on raw sockets; without the capability the check fails with a message that names the privilege |
| LB-02 | Every protocol, pass and fail, over IPv4 and IPv6; the fault rules' counters prove that the traffic crosses the router; the SSH fingerprint equals the server's host key |
| LB-03 | Every location scheme: file, `http(s)`, `https-`, `redis(s)`, `consulkv(s)`, `consulsr(s)` and `http+sso` with a real Kerberos ticket; an unknown scheme, a 404, an oversize body and a server that never answers |
| LB-04 | `--allow-net` and `--deny-net` across the two networks, names that resolve into a denied network, a redirect into one, DTLS, SSH and TLS under limits, and the size and retry bounds |
| LB-05 | Exit codes, Ctrl-C and `SIGTERM`, the output formats and templates, the defaults file search, logging, the version |
| LB-06 | While the same bundle runs, the router drops, rejects and delays and servers stop and start: each report matches the truth table of its fault |
| LB-07 | The soak |

The lab found two defects that the in-process tests could not: the package's post-install step ran `sudo setcap`, so on a system without sudo the capability was never set and ICMP checks failed; and the command waited for ever for a bundle server that accepted a connection and never answered (now `--fetch-timeout`, 30 seconds). It also confirmed the real Kerberos exchange.

## The soak

`make lab-soak` (`lab.sh soak --hours H`, 8 by default) is meant for an overnight session. For the whole time:

- the installed command runs a bundle of 200 checks against the servers about every 10 seconds, while a seeded scheduler injects one fault at a time (a dropped port, a reset, a stopped service, a delay of 4.5 s) or none, and every report is compared with the truth table of the fault; the seed is printed so that a run can be replayed;
- a race-enabled and a plain build of `TestStress` run beside it on the client; the memory and the descriptors of the plain one are sampled every minute (the race detector's own memory grows with the run and measures nothing).

The judge at the end fails on any report that does not match its truth table, on a stress binary that failed or reported a race, and on memory or descriptors that kept growing (the largest value of the last quarter over 1.5 times the median of the early window, plus 64 MiB or 32 descriptors). The records are in `build/lab/soak/`: `cycles.jsonl`, `samples.csv` and the stress logs.

## What is not covered

The lab runs on Linux with LXD and KVM only; the macOS and Windows builds are cross-compiled and unit-tested, not run against real servers. The Windows build of the command's tests uses `syscall.Kill` and does not compile there. The RPM and APK packages share the post-install script of the DEB but are not installed in the lab.
