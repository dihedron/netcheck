# netcheck: the lab

| | |
| :-- | :-- |
| Status | Approved plan, 2026-11-18; implementation in progress on `feature/lab` |
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

## 4. Threat model

| ID | Threat | Mitigation |
| :-- | :-- | :-- |
| LT-1 | Lab secrets (CA key, KDC master key, test passwords) leak into git or logs | Generated per run under `build/lab/` (git-ignored); no credential in a committed file or URL; TruffleHog and gitleaks scan the sources |
| LT-2 | The lab damages the host or the other labs | Own project, network and pool, all tagged; `down` removes only tagged objects and asks for confirmation; preflight refuses a subnet that overlaps a host route |
| LT-3 | Fault injection leaks outside the lab | nftables rules live in `nc-fw` only; the host's firewall is never touched |
| LT-4 | A long run starts in a working day and starves the machine | `soak` refuses between 08:00 and 20:00 unless told `--now`, and asks for confirmation (AGENTS.md rule 14) |
| LT-5 | The lab passes because it tests a different binary than the release | The `.deb` under test is the one built by `make snapshot`, and its checksum is recorded in the report |

## 5. Findings

- **F-1 (to confirm by LB-01 before the fix):** `packaging/postinstall.sh` runs `sudo setcap`; a minimal VM has no `sudo`, so the capability is not set and ICMP checks fail with a permission error.
