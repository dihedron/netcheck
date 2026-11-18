# Running netcheck

```text
netcheck [--format=text|json|yaml|template] [--template=FILE]
         [--print-diagnostics] [--allow-net=NET]... [--deny-net=NET]...
         [--fetch-timeout=DURATION] [BUNDLE...]
netcheck version [-v|--verbose] [-D|--dotenv FILE]
netcheck completion bash|zsh|fish|powershell
netcheck --version
```

Every argument that is not an option is the location of a bundle. They run in the order given, one after the other; the checks of one bundle run concurrently. With no bundle, `netcheck` needs a template and runs it on mock data (see "Developing a template"). Ctrl-C ends the run: the checks that had not finished are reported as skipped.

Options that take a value must be written with an equals sign (`--format=json`, `--template=my.tpl`) or glued to the short option (`-fjson`): `-f json` does not take `json` as its value.

## Bundle locations

The scheme of a location picks its source. A location with none of these schemes is a file name.

| Location | Source |
| ----------------------------------------------- | ---------------------------------------- |
| `bundle.yaml`, `/etc/checks/app.json` | A file |
| `http://host/path` | HTTP |
| `https://host/path` | HTTPS, certificate verified |
| `https-://host/path` | HTTPS, certificate **not** verified |
| `http+sso://`, `https+sso://`, `https+sso-://` | The same, authenticating with the local Kerberos or NTLM identity (`-` again skips the verification) |
| `redis://[user:pass@]host[:port]?key=K[&db=N]` | A Redis key (`rediss://` over TLS, `rediss-://` over TLS without verification) |
| `consulkv://[user:pass@]host[:port]?key=K[&dc=D]` | A Consul KV key (`consulkvs://` over TLS, `consulkvs-://` over TLS without verification) |
| `consulsr://[user:pass@]host[:port]?service=S&tag=T&meta=M[&dc=D]` | A Consul service's metadata (`consulsrs://`, `consulsrs-://` likewise) |

A trailing `-` before `://` is the equivalent of `curl -k`: use it only for tests, because it accepts any certificate. HTTP basic credentials go in the URL as usual (`https://<user>:<password>@host/bundle.yaml`). For Consul, `user:password@` is basic authentication and `:token@` (an empty user) is an ACL token.

```bash
$> netcheck local-1.yaml local-2.json \
      https://config.example.com/netcheck/payments.yaml \
      'redis://:s3cret@cache.example.com:6379?db=3&key=netcheck/payments' \
      'consulkvs://:aclToken@consul.example.com:8501?key=netcheck/payments&dc=eu1' \
      'consulsr://consul.example.com:8500?service=payments&tag=netcheck&meta=netcheck-bundle'
```

Quote a location that contains `?` or `&`, so that the shell does not interpret them. A credential on a command line is visible to other users of the host in the process list: prefer a protected bundle location that needs none, or keep the command in a file only you can read.

## Output formats

### text

The default. For each bundle a heading with its `id` is printed, then one line per check:

```text
▲    443 tcp   → www.example.com                 : web front end
▼   5432 tcp   → db.example.com                  : database  (error dialling db.example.com:5432 on protocol tcp: dial tcp 10.0.0.5:5432: i/o timeout)
```

`▲` is a pass, `▼` a failure and `◆` a check that was skipped because the run was interrupted; the line shows the port, the protocol, the host, the name of the check and, unless it passed, the reason. On a terminal the output is coloured and a spinner shows progress while a bundle runs; redirected to a file or a pipe it is plain.

### json and yaml

One document with the list of reports, one per bundle, printed when every bundle has run, so that it is always valid. A report has `bundle_id`, `description`, `started`, `duration` and `results`; a result has the `check` as it ran (with the defaults applied), `status` (`passed`, `failed`, `skipped`), `attempts`, `latency`, `error` (only when not passed) and `detail`, the evidence the protocol gathered (`remote`, `http_status`, `packets_sent`, `packets_recv`, `tls`, `ssh_banner`, `ssh_host_key`).

```bash
$> netcheck --format=json examples/netcheck.yaml | jq '.[].results[] | select(.status != "passed")'
```

### template

`--template=FILE` (which implies `--format=template`) formats the list of reports with a Go `text/template`. The template sees a list of reports; each report has:

| Field | Type |
| ------------- | -------------------------------------------------------- |
| `.BundleID`, `.Description` | string |
| `.Started` | time: `.Started.Format "15:04:05"` |
| `.Duration` | duration |
| `.Results` | the list of results |

and each result has `.Status` (`passed`, `failed` or `skipped`: compare with `eq (print .Status) "passed"`), `.Attempts`, `.Latency`, `.Err` (nil when it passed: print it with `print .Err`), `.Check` (with `.Name`, `.Address`, `.Protocol`, `.Timeout`, `.Retries`, `.Wait`, `.SSO`) and `.Detail` (with `.Remote`, `.HTTPStatus`, `.PacketsSent`, `.PacketsRecv`, `.SSHBanner`, `.SSHHostKey` and `.TLS`, which is nil unless a TLS session was made: use `with .TLS`). A template written for version 1 must be rewritten: the old `.Result.IsError` and `.Result.String` are gone. The functions of the Sprig library are available, and the colour functions `blue`, `cyan`, `green`, `magenta` (also `purple`), `red`, `yellow`, `white` and their bright versions `hiblue`, `hicyan`, `higreen`, `himagenta` (also `hipurple`), `hired`, `hiyellow`, `hiwhite`. The sample template `examples/output.tpl` uses most of them:

```{.text include="examples/output.tpl"}
```

### Developing a template

Run with the template and no bundle: the template is applied to a mock result with two reports and a few checks, passed, failed and skipped, so you can see your layout at once. Add `--print-diagnostics` to print, on the error stream, every field of the mock with the fields your template read highlighted:

```bash
$> netcheck --template=examples/output.tpl --print-diagnostics
```

## Loading time

Loading one bundle may take at most 30 seconds, whatever its source: a server that accepts the connection and never answers ends the load with `context deadline exceeded` and status 1. `--fetch-timeout=DURATION` changes the bound (`--fetch-timeout=0` removes it). The time the checks take is separate: it is set by the timeouts of the bundle.

## Exit status

| Status | Meaning |
| ------ | -------------------------------------------------------------------- |
| 0 | Every check of every bundle passed; also `--help` and `version` |
| 1 | The command line is invalid, a template or a bundle could not be loaded or is invalid, the output could not be written, or the run was interrupted |
| 2 | At least one check failed |

A script can therefore use the status directly (`netcheck bundle.yaml || alert`); for the details read `--format=json`.

## Limiting what a bundle can reach

A bundle is a list of connections the host makes on its author's behalf. When bundles come from a place you do not fully trust (a shared Redis, a web site), bound them with `--allow-net` and `--deny-net`, which take a network in CIDR form or a single address and can be repeated:

```bash
$> netcheck --allow-net=10.0.0.0/8 --deny-net=10.0.99.0/24 https://config.example.com/payments.yaml
```

A check that would connect to an address outside the allowed networks, or inside a denied one, fails. The test is made on the address actually connected to, after name resolution and redirects. A bundle may also hold at most 1,000 checks, 10 attempts per check, 256 concurrent checks and a 5-minute timeout. With either option, HTTP checks do not use a proxy, and checks with `sso: true` are refused. Without them there is no restriction.

## Version

`netcheck version` prints one line with the name, the version and the Go version; `netcheck version -v` prints the build details (version, commit, build time, licence). `--dotenv FILE` also loads a `.env` file and, in the verbose output, lists the `NETCHECK_*` variables with the values of passwords, tokens and secrets masked.

## Completion and the manual page

`netcheck completion SHELL` prints the completion script of `bash`, `zsh`, `fish` or `powershell` (chapter 2 says where to put it); an unknown shell is a usage error, status 1. The command is described in the manual page `netcheck(1)`, which has the options, the bundle locations, the exit status, the files and the environment variables of this chapter in the form `man` shows.

## Logging and profiling

Logging is off by default: the program writes only its results. Set environment variables to turn it on. In the names below the prefix is the name of the binary in capitals, with a hyphen replaced by an underscore (`NETCHECK` for `netcheck`).

| Variable | Values | Effect |
| ------------------- | ----------------------------------- | -------------------------------------- |
| `NETCHECK_LOG_LEVEL` | `debug`, `info`, `warn`, `error`, `off` | The level; unset means off |
| `NETCHECK_LOG_STREAM` | `stderr` (default), `stdout`, `file` | Where logs go; `file` writes `netcheck-<pid>.log` |
| `NETCHECK_CPU_PROFILE` | a file name | Write a CPU profile there |
| `NETCHECK_MEM_PROFILE` | a file name | Write a memory profile there |
| `NETCHECK_DOTENV` | a file name | Load that `.env` file first; it may set the variables above |

At `debug` the log shows each attempt of each check, with its latency and error, and the loading of each bundle. Credentials in addresses and locations are removed from the log. The content of a fetched bundle is not logged.
