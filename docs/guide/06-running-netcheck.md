# Running netcheck

```text
netcheck [--format=text|json|yaml|template] [--template=FILE]
         [--print-diagnostics] [BUNDLE...]
netcheck version [-v|--verbose] [-D|--dotenv FILE]
netcheck --version
```

Every argument that is not an option is the location of a bundle. They run in the order given, one after the other; the checks of one bundle run concurrently. With no bundle, `netcheck` runs on mock data (see "Developing a template").

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

`▲` is a success and `▼` a failure; the line shows the port, the protocol, the host, the name of the check and, for a failure, the error. On a terminal the output is coloured and a spinner shows progress while a bundle runs; redirected to a file or a pipe it is plain.

### json and yaml

One document with the list of bundles, printed when every bundle has run, so that it is always valid. Each bundle has its settings and its `checks`, and each check its settings and its `result`: the text `success` or the error.

```bash
$> netcheck --format=json examples/netcheck.yaml | jq '.[].checks[] | select(.result != "success")'
```

### template

`--template=FILE` (which implies `--format=template`) formats the list of bundles with a Go `text/template`. The template sees a list of bundles; each bundle has:

| Field | Type |
| ------------- | -------------------------------------------------------- |
| `.ID`, `.Description` | string |
| `.Timeout`, `.Wait` | duration: print with `.Timeout.String` |
| `.Retries`, `.Concurrency` | number |
| `.Checks` | the list of checks |

and each check has `.Name`, `.Address`, `.Protocol` (print it with `.Protocol.String`), `.Timeout`, `.Retries`, `.Wait`, `.SSO` and `.Result`, which has `.IsError` (true for a failure) and `.String` (`success` or the error). The functions of the Sprig library are available, and the colour functions `blue`, `cyan`, `green`, `magenta` (also `purple`), `red`, `yellow`, `white` and their bright versions `hiblue`, `hicyan`, `higreen`, `himagenta` (also `hipurple`), `hired`, `hiyellow`, `hiwhite`. The sample template `examples/output.tpl` uses most of them:

```{.text include="examples/output.tpl"}
```

### Developing a template

Run with the template and no bundle: the template is applied to a mock result with two bundles and a few checks, with successes and failures, so you can see your layout at once. Add `--print-diagnostics` to print, on the error stream, every field of the mock with the fields your template read highlighted:

```bash
$> netcheck --template=examples/output.tpl --print-diagnostics
```

## Exit status

| Status | Meaning |
| ------ | -------------------------------------------------------------------- |
| 0 | The run completed, **whether or not checks failed**; also `--help` and `version` |
| 1 | The command line is invalid, a template is missing, or a bundle could not be loaded |

In this version a failed check does not change the exit status: to use `netcheck` in a script, read the output (`--format=json`) and look for results other than `success`.

## Version

`netcheck version` prints one line with the name, the version and the Go version; `netcheck version -v` prints the build details (version, commit, build time, licence). `--dotenv FILE` also loads a `.env` file and, in the verbose output, lists the `NETCHECK_*` variables with the values of passwords, tokens and secrets masked.

## Logging and profiling

Logging is off by default: the program writes only its results. Set environment variables to turn it on. In the names below the prefix is the name of the binary in capitals, with a hyphen replaced by an underscore (`NETCHECK` for `netcheck`).

| Variable | Values | Effect |
| ------------------- | ----------------------------------- | -------------------------------------- |
| `NETCHECK_LOG_LEVEL` | `debug`, `info`, `warn`, `error`, `off` | The level; unset means off |
| `NETCHECK_LOG_STREAM` | `stderr` (default), `stdout`, `file` | Where logs go; `file` writes `netcheck-<pid>.log` |
| `NETCHECK_CPU_PROFILE` | a file name | Write a CPU profile there |
| `NETCHECK_MEM_PROFILE` | a file name | Write a memory profile there |
| `NETCHECK_DOTENV` | a file name | Load that `.env` file first; it may set the variables above |

At `info` the log shows each successful check, with the certificate issuer and expiry date for TLS checks; at `warn`, each failed attempt; at `debug`, much more, including the contents of the fetched bundle. Do not leave `debug` on where the logs are kept: bundle locations and what they return can contain credentials.

## Troubleshooting

| Symptom | Likely cause |
| ------------------------------------------ | ---------------------------------------------------------- |
| `Cannot load package from ...: no such file` | The location is not a recognised scheme and is not a file; check the spelling of the scheme |
| `invalid key`, `invalid service` | The `key`, or the `service`, `tag` and `meta` query parameters of the location are missing |
| `unsupported or undetected format` | The Redis or Consul value does not start with `---`, `{` or `[` |
| An ICMP check fails with a permission error | The binary lacks `cap_net_raw` (chapter 3) |
| A TLS check fails on a host name | The address uses an IP address, or a name the certificate does not carry |
| A bundle never ends | `concurrency: 0` in the bundle (chapter 4) |
| `-f json` complains about a missing bundle | Write `--format=json` or `-fjson` |
