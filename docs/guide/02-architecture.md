# Architecture

![From the command line to the printed result](architecture.pdf){width=100%}

## The pieces

| Piece | Role |
| ------------------- | -------------------------------------------------------------------------- |
| Command | Parses the command line, loads each bundle, runs it, prints the result |
| Bundle sources | Fetch the text of a bundle from a file, an HTTP(S) server, Redis or Consul and tell its format |
| Bundle | The parsed document: identity, defaults (timeout, retries, wait, concurrency) and the list of checks |
| Library | `pkg/netcheck`: the worker pool (at most `concurrency` checks at a time) and one runner per protocol, each attempt bounded by the timeout, failures retried; `pkg/netcheck/fetch` and its sub-packages load bundles. The command is a thin user of it |
| Output | Text, JSON, YAML or a user template, with colours where the terminal supports them |

## A run, step by step

1. The command receives zero or more bundle locations. With none, it runs on mock data, which is meant for developing templates (chapter 6).
2. For each location, in order, the matching **source** fetches the text. The scheme of the location picks the source (chapter 6); anything that is not a recognised URL is a file name.
3. The **format** is hinted by the source: from the file extension or from the `Content-Type` header of an HTTP response; without a hint the content decides (a document starting with `---` is YAML, one starting with `{` or `[` is JSON, anything else is tried as JSON and then as YAML).
4. The bundle is parsed. Values the bundle does not set come from the defaults (below).
5. The checks are queued for a pool of `concurrency` workers. Each worker takes a check, runs it with its timeout, and on failure waits and retries, up to `retries` attempts in total.
6. When all checks of the bundle are done, the results are printed (text format) or kept until every bundle has run (JSON, YAML, template), so that the output is one valid document.

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

The defaults can be changed with a file `netcheck.conf` (chapter 3). The package installs one that sets a 2 s timeout and a 100 ms wait. `concurrency`, `ping` and the other settings that are not in the table apply to the whole run: they are not set per check.

## Results

Each check ends as **passed**, **failed** or **skipped** (the run was interrupted before it finished). A result carries the number of attempts, the latency of the last one, the error of the last attempt for a failure (a refused connection, a timeout, a certificate that has expired, a host name that does not match) and the evidence the protocol gathered: the resolved address, the HTTP status, the packets of a ping, the certificate, the SSH fingerprint. The command logs each attempt at the `debug` level (chapter 6).

## Templates and field tracking

When you give a template, `netcheck` executes it on the list of reports, one per bundle. To help you write one without waiting for real checks, running with a template and no bundle uses a mock result whose fields count how often the template reads them, and `--print-diagnostics` prints which fields your template used (chapter 6).
