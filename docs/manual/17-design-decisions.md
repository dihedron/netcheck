# Design decisions

::: landscape

| Decision | Choice | Why |
| ------------------------------ | ------------------------------------------- | ---------------------------------------------- |
| One module, many packages | The core is standard library plus three small modules; Redis, Consul, DTLS and Kerberos are sub-packages | A consumer pays only for what it links (T-11) |
| Registration by the caller | `WithRunner`, `fetch.WithSource`; no `init()` | What a binary can do is visible in its code; no global state |
| Module path `/v2` | The release is v2.0.0 | The module changed its API and output; v1 tags exist |
| Output shape | The command prints the library's `Report`; the old output shape was dropped | One shape for the command and the library beats an adapter; v2 is the moment to break |
| TLS options only in the API | The bundle file has no TLS fields | The file format stays small and is not a way to weaken verification |
| `Run` and `RunCheck` | Both | A client SDK wants one check; a command wants a bundle |
| UDP grace window | `Defaults.UDPGrace`, 200 ms, no per-check field | A dial alone proves nothing; the check stays best effort |
| SSH host key | The fingerprint is returned, not verified | Pinning needs a place to keep the pin; planned |
| Limits in the library | Default bounds in `Validate`, networks in options | A bundle from an untrusted place must not exhaust or scan the host |
| Exit status | 0 passed, 1 error or interrupted, 2 checks failed | A script can tell a broken run from a failed network |
| Backward compatibility | None kept for v1 | The product has no v1 users to protect, and the old code had defects that compatibility would preserve |

:::

The specifications are in `.specs/` of the source repository: `netcheck-library.md` (the library, its threat model and acceptance criteria) and `netcheck-reverse-engineering.md` (the defects of the code that was replaced, all fixed).
