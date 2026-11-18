# Operating netcheck

## In a script, a pipeline or a probe

- Use the exit status (chapter 6): `netcheck bundle.yaml || alert`. Status 2 means that checks failed, status 1 that `netcheck` itself could not do its work; a probe should tell the two apart.
- Write `--format=json` to a file when the outcome must be kept: it is a single valid document even when several bundles ran.
- A run that is stopped (Ctrl-C, `SIGTERM`, a job timeout that sends it) prints what it has, marks the unfinished checks as skipped and exits with status 1.
- Put a bound on the total time: a bundle takes at most `retries` × `timeout` + (`retries` - 1) × `wait` for its slowest check, and its checks run `concurrency` at a time. A bundle of 100 checks that all time out, with a 2 s timeout, 3 attempts, a 100 ms wait and 10 workers, takes about 62 s.

## Bundles from places you do not trust

A bundle is a list of connections the host makes on its author's behalf: a bundle in a shared Redis or on a web site can turn the host into a scanner of its own network. Bound the reach with `--allow-net` and `--deny-net` (chapter 6). The command also refuses what would exhaust the host: more than 1,000 checks, 10 attempts, a timeout over 5 minutes or 256 workers.

Credentials in the user-info part of a location (a user name and password before the host) are used and never logged, but they are visible in the process list: prefer the environment or a client certificate where the source supports it.

## Keeping an eye on a run

Set `NETCHECK_LOG_LEVEL=debug` to see each attempt of each check with its latency and error (chapter 6). `NETCHECK_CPU_PROFILE` and `NETCHECK_MEM_PROFILE` write profiles for a developer to read.

## Troubleshooting

| Symptom | Likely cause |
| ------------------------------------------ | ---------------------------------------------------------- |
| `Cannot load the bundle: no source registered for scheme "..."` | The location has a scheme `netcheck` does not know; check its spelling |
| `the location has no key`, `needs service, tag and meta` | The `key`, or the `service`, `tag` and `meta` query parameters of the location are missing |
| `unsupported or undetected format` | The Redis or Consul value does not start with `---`, `{` or `[` |
| An ICMP check fails with a permission error | The binary lacks `cap_net_raw` (chapter 2), or `ping.privileged: false` and your group is outside `net.ipv4.ping_group_range` |
| A TLS check fails on a host name | The address uses an IP address, or a name the certificate does not carry |
| `-f json` complains about a missing bundle | Write `--format=json` or `-fjson` |

## Error reference

| Error text contains | Meaning |
| ------------------------------------------ | -------------------------------------------------------- |
| `connection refused` | The host answered and nothing listens on that port, or a firewall rejects |
| `i/o timeout`, `context deadline exceeded` | Nothing answered within the timeout: a firewall that drops packets, a dead host, a slow server |
| `no such host` | The name does not resolve |
| `no reply from ...: 100% loss` | The icmp check got no echo reply |
| `network not allowed` | `--allow-net` or `--deny-net` forbids the address that the name resolved to or a redirect led to |
| `certificate has expired`, `certificate signed by unknown authority`, `certificate is valid for ...` | The TLS check verified the certificate and it is not acceptable |
| `redirect from https to http refused (downgrade)` | An https check was redirected to plain http; the command always refuses it |
| `stopped after 10 redirects` | A redirect loop, or a chain longer than 10 |
| `answered 404 Not Found` | The HTTP check got a status outside 200 to 399 |
| `pinging ...: ... permission` (icmp) | The binary lacks `cap_net_raw` (chapter 2) |
| `SSH handshake with ...` | The port does not speak SSH, or the server closed the key exchange |
