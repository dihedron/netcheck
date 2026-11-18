# netcheck: the lab

| | |
| :-- | :-- |
| Status | Implemented 2026-11-18 on `feature/lab`; the first overnight soak ran the same night (results in §6) |
| Basis | `netcheck-library.md` (§8.8 lab test, §14 audit, the open gaps: real Kerberos, ICMP pass path), AGENTS.md rules 10 and 14 |
| Goal | Run the installed package against **real servers** for every scenario of the manual, prove the package sets `cap_net_raw`, and prepare the overnight soak |

## 1. Topology

LXD virtual machines in their own project (`netcheck-lab`), network (`nclab`, `10.77.0.0/24`, NAT, IPv6 ULA) and storage pool, every object tagged `user.netcheck-lab=yes`, so that the lab never touches the ligo lab or anything else on the host.

| VM | Role |
| :-- | :-- |
| `nc-servers` | The real servers: nginx (HTTP, HTTPS with the lab CA, redirects, 404, 500, slow, https to http, a redirect loop, a large body), OpenSSH, TCP and UDP listeners (`socat`), `openssl s_server -dtls`, TLS listeners with an expired and a wrong-name certificate, Redis, Consul (KV and a registered service), a Kerberos KDC with nginx `auth_gss` for SSO, dnsmasq |
| `nc-client` | Ubuntu 24.04 without sudo or a toolchain: the `.deb` is installed with `dpkg -i`, as a user would; checks run as an unprivileged user and as root |
| `nc-fw` | A router between the client's subnet and the servers' second subnet, with nftables, to inject drops, rejects and delay |

## 2. Commands

`test/lab/lab.sh`: `preflight`, `up`, `down`, `status`, `deploy`, `test`, `soak`, `ssh`, `logs`. Settings in `lab.env`, overridden by `lab.local.env` (git-ignored). State (generated CA, keys, keytab) under `build/lab/`, never in git.

## 3. Scenarios (`test/lab/acceptance`, `-tags lab`, driving the installed binary over `lxc exec`)

| ID | Scenario |
| :-- | :-- |
| LB-01 | Package: `getcap /usr/bin/netcheck` shows `cap_net_raw`; as an unprivileged user an `icmp` check passes on raw sockets; after `setcap -r` it fails with a message that names the privilege; an upgrade keeps the capability and an edited `/etc/netcheck.conf` |
| LB-02 | Every protocol, pass and fail, against the real servers, IPv4 and IPv6 |
| LB-03 | Every location scheme, including `+sso` against the KDC (the real Kerberos exchange), 404, oversize and hanging sources |
| LB-04 | Network limits across the two subnets, a redirect leading into a denied network, the bundle size and retry bounds |
| LB-05 | The command: exit codes, interruption, output formats, templates, defaults file, logging |
| LB-06 | Faults: while bundles loop, the router drops, rejects and delays and servers stop and start; each report matches the truth table of the moment |
| LB-07 | Soak (overnight): hours of LB-06 with the sampled resource use judged at the end |
| LB-09 | DTLS (`lab.sh dtls-stress`, minutes): cycles of abandoned handshakes (every packet 4.5 s late) each followed by a calm cycle with the installed command, and a probe that abandons handshakes and reuses their source ports against the server; no check or round may fail |

## 4. Threat model

| ID | Threat | Mitigation |
| :-- | :-- | :-- |
| LT-1 | Lab secrets (CA key, KDC master key, test passwords) leak into git or logs | Generated per run under `build/lab/` (git-ignored); no credential in a committed file or URL; TruffleHog and gitleaks scan the sources |
| LT-2 | The lab damages the host or the other labs | Own project, network and pool, all tagged; `down` removes only tagged objects and asks for confirmation; preflight refuses a subnet that overlaps a host route |
| LT-3 | Fault injection leaks outside the lab | nftables rules live in `nc-fw` only; the host's firewall is never touched |
| LT-4 | A long run starts in a working day and starves the machine | `soak` refuses between 08:00 and 20:00 unless told `--now`, and asks for confirmation (AGENTS.md rule 14) |
| LT-5 | The lab passes because it tests a different binary than the release | The `.deb` under test is the one built by `make snapshot`, and its checksum is recorded in the report |

## 5. Findings

- **F-1 (fixed):** `packaging/postinstall.sh` ran `sudo setcap`; a minimal VM has no `sudo`, so the capability was not set and ICMP checks failed with a permission error. LB-01 failed first (against the real .deb), then passed: the script calls `setcap` directly, reports a failure on stderr without failing the install, and the packages depend on setcap's package (`libcap2-bin`, `libcap`, `libcap-utils`).
- **F-2 (fixed):** a bundle server that accepted the connection and never answered hung the command for ever (T-7 relied on the caller's context). `--fetch-timeout` (30 s, 0 for none) bounds the load; `TestRunFetchTimesOut` and the lab's `TestSourceHangingServer`.
- **F-3 (lab, not the product):** the first soaks failed on the lab's own servers: `openssl s_server` serves one DTLS session at a time (replaced by `test/lab/dtlsd`), sshd's `MaxStartups` dropped concurrent handshakes, Ubuntu's `ssh.socket` kept port 22 open after the service stopped, and the race detector's own memory looked like a leak (the memory judge reads the plain build).
- **F-4 (lab, not the product):** the soak's only failures (3 in 3,744 cycles) were DTLS checks that timed out after one attempt, in cycles that followed a `delay 4.5 s` cycle. A capture on nc-fw showed the client's ClientHello forwarded three times (retransmissions at 0, 1 and 3 s) and the server never answering; the source port had been used 10 s earlier by the abandoned handshake. pion's listener routes every datagram of a source address to the connection that address had first, until it closes, so the stale handshake swallowed the new client's. Reproduced by `test/lab/dtlsprobe` (42 of 300 rounds failed), fixed in `test/lab/dtlsd` (its own listener starts a new connection when a ClientHello with a different random arrives from a known address; unit tests and a fuzz target run on loopback with `go test`), then 0 failures in 6,000 rounds. The check's own retransmissions behaved correctly: the product was not at fault.

## 6. Results

Acceptance (`lab.sh test`): green on a clean lab, all scenarios LB-01 to LB-06.

Soak (LB-07), 2026-11-18:

- 6 h run, failed with 177 mismatches. Cause: the race-enabled stress binary's resident set grew by gigabytes in minutes (race detector shadow memory; the plain binary stayed flat at about 33 MB) and starved the client VM, so every check timed out. Static routes added with `ip` were also dropped by DHCP renewals. Both are lab faults, fixed: the race binary now runs in 4-minute slices, and the routes are in netplan.
- 30 min re-run: both stress binaries pass, race resident set at most 1.3 GiB, plain flat; one mismatch in about 300 cycles: a single DTLS check failed in a calm cycle (the same kind of event happened 3 times in about 2,700 cycles of the 6 h run). Not reproduced in 400 runs of a DTLS-only bundle or 300 runs of the full bundle outside the soak. Open: the soak now records the failing check's error, to be read in the next run.
- 6 h run on the cli-extras build (3,744 cycles): both stress binaries pass (plain resident set at most 56 MB, at most 2,248 descriptors, no growth; race binary at most 1.35 GiB); 3 mismatches, all the DTLS check in a cycle after a `delay 4.5 s` cycle: F-4, a fault of the lab's server, fixed.
- `lab.sh dtls-stress` (LB-09) on the fixed server: 150 abandoned-handshake cycles with the installed command and 6,000 probe rounds of reused source ports, no failure (against the old server the probe failed 42 of 300 rounds). The full soak was not repeated: the only failure it showed is covered by LB-09, which fails on the old server.
