# Threat model

**Assets:** the host the checks run from (a workload or an operator's machine); credentials in bundle locations; the trustworthiness of a check's outcome.

The table gives each threat and the mitigation **as built**; the last column names the test that enforces it.

::: landscape

| ID | Threat | Mitigation | Evidence |
| :-- | :------------------ | :------------------------------------------ | :------------------ |
| T-1 | A hostile or compromised bundle source feeds checks that make the tool a scanner or an SSRF probe against internal addresses | `WithAllowedNetworks` and `WithDeniedNetworks` vet the resolved address at dial time; `--allow-net` and `--deny-net` in the command; chapter 8 says when to use them | `networks_test.go` |
| T-2 | A bundle with huge retries, timeout, concurrency or size exhausts the host | Bounds in `Validate`: 10 attempts, 5 min, 256 workers, 1,000 checks; `WithLimits` changes them explicitly | `validate_test.go`, `TestStress` |
| T-3 | Credentials leak through logs or errors | Locations and addresses are redacted before they are logged or wrapped; bodies are never logged | `TestLogsAreRedacted` |
| T-4 | TLS verification disabled by accident | Verification is on; switching it off needs `TLSOptions.InsecureSkipVerify` or the command's explicit `-` scheme; the result records `verified: false` | `tcp_tls_test.go` |
| T-5 | A check reports success when nothing was proven | Each protocol's guarantee is stated (chapter 11); `detail` carries the evidence | per-protocol tests |
| T-6 | An SSH check is taken for host authentication | Documented as reachability and banner only; the fingerprint is returned for the caller to pin | `ssh_udp_icmp_test.go` |
| T-7 | A remote source returns a gigantic or endless body | `fetch.WithMaxSize` (1 MiB by default) and the context's deadline on every fetch | `fetch_test.go` |
| T-8 | Redirects lead elsewhere or from https to http | At most 10 redirects; a downgrade fails the check; with network limits every hop is vetted | `http_test.go` |
| T-9 | A data race or shared state between concurrent checks and the host program | No globals; the caller's `http.Client` is copied before a check changes its transport; `-race` in CI | `go test -race`, `guard_test.go` |
| T-10 | A runner panics or hangs and stalls the bundle | Recover per check; every runner receives a context with a deadline | `run_test.go`, `TestStress` |
| T-11 | Supply chain: the library drags large dependency trees into every consumer | Sub-packages; the import allow-list test; `govulncheck`, dependency review, TruffleHog and gitleaks in CI; signed checksums for every release file | `TestImportAllowList` |
| T-12 | ICMP raw sockets need privileges | Privilege is the caller's choice; a missing privilege is an error that says so; the package grants `cap_net_raw` to the command only | `ssh_udp_icmp_test.go` |

:::

## Residual risks

- **SSH host keys are not verified** (T-6): a man in the middle between the host and the server cannot be detected by the check. Fingerprint pinning is planned.
- **UDP is best effort** (T-5): a firewall that drops packets and a quiet service look the same.
- **SSO cannot be combined with network limits or TLS options**: the SSO client is used as given, so the limits would not bind it. The check is refused with a clear error instead of running unbounded.
- **Credentials in a location are visible in the process list** of the host while the command runs.
- **The bundle is trusted to describe checks, not to be safe**: without limits, a bundle can make the host connect anywhere it can reach. This is the purpose of the tool; the limits exist for bundles from places that are not fully trusted.
