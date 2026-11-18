# A first run

Write a bundle, `first.yaml`:

```yaml
id: first-run
description: Is the network what I think it is?
retries: 2
checks:
  - name: dns over tcp
    address: 1.1.1.1:53
  - name: example web site
    protocol: https
    address: example.com
  - name: a closed port
    address: 127.0.0.1:9
```

Run it:

```bash
$> netcheck first.yaml
```

Each check is shown as passed, failed or skipped, with the number of attempts and the latency; a failed check shows the error of its last attempt (for the closed port, a refused connection). The status of the command tells a script what happened:

| Status | Meaning |
| ------ | ------------------------------------------------------------ |
| 0 | Every check passed |
| 1 | The command line, a bundle or a template is invalid, or the run was interrupted |
| 2 | At least one check failed |

The closed port above fails on purpose, so this run ends with status 2.

## Next steps

- `netcheck --format=json first.yaml` prints the result for a program to read.
- `netcheck --allow-net=10.0.0.0/8 https://config.example.com/payments.yaml` runs a bundle from a server, connecting only inside the given network.
- `netcheck version -v` prints the version and the build details; a bug report should carry them.
- Chapters 4 to 6 describe every check, every place a bundle can live and every output format.
