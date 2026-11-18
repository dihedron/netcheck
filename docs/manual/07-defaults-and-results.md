# Defaults and results

## Defaults and overrides

Every value that can be given in a check can be given in the bundle, and every value of the bundle has a default. A check inherits from its bundle, a bundle from the defaults:

| Setting | Meaning | Compiled-in default |
| ----------- | ------------------------------------------------------------ | ----------------- |
| `timeout` | How long one attempt may take | 20 s |
| `retries` | How many attempts in total before a check is reported as failed | 3 |
| `wait` | Pause between two attempts | 1 s |
| `concurrency` | How many checks of a bundle run at the same time | 10 |
| `ping.count` | ICMP echo requests per attempt | 10 |
| `ping.interval` | Pause between ICMP echo requests | 100 ms |
| `ping.size` | ICMP payload size in bytes | 64 |
| `ping.privileged` | Use raw ICMP sockets (needs `cap_net_raw` or root) | true on Linux and Windows |

The defaults can be changed with a file `netcheck.conf` (chapter 2). The package installs one that sets a 2 s timeout and a 100 ms wait. `concurrency`, `ping` and the other settings that are not in the table apply to the whole run: they are not set per check.

## Results

Each check ends as **passed**, **failed** or **skipped** (the run was interrupted before it finished). A result carries the number of attempts, the latency of the last one, the error of the last attempt for a failure (a refused connection, a timeout, a certificate that has expired, a host name that does not match) and the evidence the protocol gathered: the resolved address, the HTTP status, the packets of a ping, the certificate, the SSH fingerprint. The command logs each attempt at the `debug` level (chapter 6).

## Results

Each check ends as **passed**, **failed** or **skipped** (the run was interrupted before it finished: a skipped check is not a pass). A result carries the number of attempts, the latency of the last one, the error of the last attempt for a failure (a refused connection, a timeout, a certificate that has expired, a host name that does not match) and the evidence the protocol gathered, in `detail`:

| Field of `detail` | Set by | Content |
| ------------------- | --------------------- | ------------------------------------------------- |
| `remote` | every protocol | The address actually connected to, after name resolution |
| `http_status` | `http`, `https` | The status of the final response |
| `packets_sent`, `packets_recv` | `icmp` | Echo requests sent and replies received |
| `tls` | `tls`, `dtls`, `https` | Version, cipher suite, subject, issuer, expiry, names, and `verified` |
| `ssh_banner`, `ssh_host_key` | `ssh` | The server's version string and the SHA-256 fingerprint of its host key (not verified) |

The command writes one **report** per bundle: its `bundle_id`, `description`, start time and `duration`, and the list of `results` in the order of the bundle's checks. JSON and YAML output is the list of reports, durations as strings (`"211ms"`):

```json
[{"bundle_id": "sample", "duration": "211ms", "started": "2026-11-18T00:14:36+01:00",
  "results": [{"check": {"name": "closed", "address": "127.0.0.1:9", "protocol": "tcp",
                         "timeout": "2s", "retries": 1, "wait": "100ms", "sso": false},
               "status": "failed", "attempts": 1, "latency": "154us",
               "error": "connecting to 127.0.0.1:9: connection refused", "detail": {}}]}]
```

The `check` is the check as it ran, with the defaults applied.

## Templates and field tracking

When you give a template, `netcheck` executes it on the list of reports. A template sees `.BundleID`, `.Description`, `.Started`, `.Duration` and `.Results`, each result with `.Check`, `.Status`, `.Attempts`, `.Latency`, `.Error` and `.Detail`. To help you write one without waiting for real checks, running with a template and no bundle uses a mock result whose fields count how often the template reads them, and `--print-diagnostics` prints which fields your template used (chapter 6).
