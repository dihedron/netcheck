# netcheck: backlog

Work agreed for later, with what is known so far. An item here is not a specification: each needs its own, approved, before any code (AGENTS.md rule 1). Update this file when an item is added, started or done.

| No. | Item | Recorded | Status |
| :-- | :-- | :-- | :-- |
| 1 | A man page and shell completions (bash, zsh, fish, PowerShell), before v2.0.0 | 2026-11-18 | Done in v2.0.0 (PR #34): `.specs/netcheck-cli-extras.md` |
| 2 | One clean overnight soak (6 h) before the v2.0.0 tag | 2026-11-18 | Done: the 6 h run's only failures were the lab's DTLS server (F-4), fixed; `lab.sh dtls-stress` passes 150 cycles and 6,000 rounds (`.specs/netcheck-lab.md` §6) |
| 3 | `ligo` depends on netcheck pinned to a commit; bump to v2.0.0 after the release | 2026-11-18 | Done: `ligo` requires v2.0.0 since 2026-11-18 (`ligo` PR #62) |
| 4 | Environment variables in bundle locations, so that credentials need not be written in a URL | 2026-11-18 | Not started; see §4 |
| 5 | DTLS 1.3 | 2026-11-18 | Not started; waits for a library that speaks it; see §5 |
| 6 | A daemon mode: `netcheck serve` runs the bundles every so often, as a systemd unit, with signals and a configuration reload | 2026-11-18 | Not started; see §6 |
| 7 | Hooks into the check, and the `postgresql` and `spire` checks (v2.1.0) | 2026-11-19 | Built on `feature/tls-kinds`, release v2.1.0 after the merge: `.specs/netcheck-tls-kinds.md` |
| 8 | Per-check `allow_nets` and `deny_nets` in a bundle file; they narrow, never widen, the command line's | 2026-11-19 | Not started |

## 4. Environment variables in bundle locations

**Why.** A bundle kept on a server often needs credentials in its location: the user information of an `https://` URL (`https://[USER[:PASSWORD]@]HOST/PATH`), or the same part of a `redis://` or `consulkv://` one. Written on the command line they show in the process list and the shell history. The wish is to write `${MY_USERNAME}` and `${MY_PASSWORD}` where the user name and the password go, and have the command read the two values from its environment, so that the unit file, the script or the defaults file holds names and the secrets live in an environment file or a secret store.

**What makes it hard.**

- A value is data, not syntax. Put into a URL as it is, a password with `@`, `:`, `/`, `?` or `#` changes what the URL means, and a value of `evil.example@` in the user name could change the host. The expansion has to happen after the URL is parsed, by component, with each value escaped for the place it lands in (user name, password, path segment, query value).
- The expanded location is a secret. The report, the diagnostics, the logs and the error messages must show the location as it was written, with the variable names, as they scrub the password of a location today.
- An unset variable is an error that names the variable, never an empty string; a variable in the host part needs a decision (allowed, and the network limits apply to the result as they do now, or refused).
- One syntax, `${NAME}`, with a way to write a literal `$`; no `$NAME`, no default values, no command substitution, nothing a shell would do.

**Questions for the spec.** Which places expand: the locations on the command line only, or also the ones in the defaults file and, later, the daemon's configuration; whether the `.env` file that `NETCHECK_DOTENV` loads is a source of the variables; whether the same applies to the addresses of the checks (probably not: a bundle is data that may come from elsewhere, and it must not read the caller's environment).

## 5. DTLS 1.3

**Why.** The `dtls` check speaks DTLS 1.2 only. DTLS 1.3 (RFC 9147) is the current version, and a server that has dropped 1.2 cannot be checked.

**What makes it hard.** The `dtls` runner is built on `pion/dtls` v3, which implements 1.2 and no later version, and the standard library has no DTLS. The work waits for a library that speaks 1.3, or for the decision to write one, which is out of proportion for this tool. The lab's server (`test/lab/dtlsd`) is also built on pion, so a 1.3 server for the lab is a second thing to find.

**Direction.** Check where pion's 1.3 work stands, and what other libraries exist, when the item is taken up. The check should negotiate the highest version both sides speak, report it in `Detail.TLS.Version` (it says "DTLS 1.2" today), and keep working against 1.2-only servers; a bundle may need a way to pin the version, as the `tls` check's options do for TLS. The core package must keep not importing the DTLS runner.

## 6. A daemon mode

**Why.** Today `netcheck` runs the bundles once and exits, and cron or a timer repeats it. A host that is checked all day is better served by a long-lived process: it can run each bundle every X seconds, keep the last result, and be controlled like any service.

**Direction (to be confirmed in the spec).**

- A process command, `netcheck serve` (the AGENTS.md convention for the commands that run a process), that runs the bundles at an interval, set for all of them or for each one. It runs in the foreground, as a systemd unit expects (`Type=notify` or `simple`); it does not fork and detach itself, which a unit does not need and which makes signals and logs harder. The package installs a `netcheck.service`, with `AmbientCapabilities=CAP_NET_RAW` for the ICMP check and the usual hardening, and an `EnvironmentFile=` for the credentials of item 4.
- Signals: `SIGHUP` reloads the configuration (the bundles, the interval, the defaults file), and a configuration that does not load keeps the running one and says why; `SIGTERM` and `SIGINT` stop after cancelling the run in progress, as Ctrl-C does now, reporting the checks not finished as skipped; perhaps `SIGUSR1` to run the bundles now. With systemd, `sd_notify` for `READY`, `RELOADING` and `STOPPING`, and the watchdog.
- A run that is still going when the next is due is not overlapped: it is skipped and logged.

**Open questions.** Where the results go: the journal as structured log lines, a file in the formats and templates of today, an HTTP endpoint with the last report or Prometheus metrics, a webhook on a change of state; what the exit status of a stopped daemon means; how the daemon's own configuration is written (a file beside `netcheck.conf`), and whether a reload may add or remove bundles; how the memory and descriptors of a process that runs for weeks are judged (the soak already does that for the library).

**Threat model to write.** The reload path (a bad or hostile configuration file, a partial write), the privilege of the unit (an unprivileged user with only the ICMP capability), the exposure of the results if an endpoint is added, and the credentials in the unit's environment.

**Lab.** A scenario with the installed unit: `systemctl reload` and `kill -HUP`, a bad configuration during a reload, `systemctl stop` during a run, and a soak of the daemon against the faults of the lab.
