# A man page and shell completions

- **Version:** 0.1 (2026-11-18), approved by the user in plan mode
- **Backlog:** `.specs/netcheck-backlog.md`, item 1
- **Branch:** `feature/cli-extras`, from `feature/lab`

## 1. Why

netcheck has seven options, bundle sources as arguments and a `version` command, but no man page and no shell completion. Both are expected of a command that is installed from a package. They ship before v2.0.0.

## 2. Decisions (the user, 2026-11-18)

| Question | Decision |
| :-- | :-- |
| How | A `completion <shell>` command prints the script for bash, zsh, fish or PowerShell, generated from the live option parser. The release writes the files and the packages install them. |
| Man page | Hand-written Markdown (`docs/man/netcheck.1.md`), built with the pandoc of the docs pipeline into `docs/man/netcheck.1`, which is committed; `make man-check` fails when it is stale. No `man` command: the page is a packaging artifact. |
| Command line rule | `completion` is an exception to `<object> <verb>`, as `version`, `serve` and `migrate` are: it is a process command. |

## 3. Design

- `netcheck completion SHELL` is answered before the flat command line is parsed, as `version` is. Unknown shell or no shell: a usage error, exit 1.
- The scripts are static. The options, their short and long names, their descriptions and the `--format` choices come from the parser; the bundle arguments and `--template` complete file names. Pressing TAB never runs netcheck.
- Files in the packages: `/usr/share/bash-completion/completions/netcheck`, `/usr/share/zsh/site-functions/_netcheck`, `/usr/share/fish/vendor_completions.d/netcheck.fish`, `/usr/share/man/man1/netcheck.1.gz`. The archives carry the man page and the four scripts; PowerShell users run `netcheck completion powershell`.
- The man page has NAME, SYNOPSIS, DESCRIPTION, OPTIONS, BUNDLE SOURCES, EXIT STATUS, FILES, ENVIRONMENT, EXAMPLES, SEE ALSO and AUTHOR.

## 4. Threat model

| ID | Threat | Mitigation |
| :-- | :-- | :-- |
| T-K1 | TAB runs the tool or a shell snippet, so a completion opens a connection. | The scripts are static and call no command; a test rejects command substitution and `eval` in them. |
| T-K2 | An option description breaks a script or injects code. | All text comes from the binary's own option tags, escaped by one function per shell; a fuzz target feeds it hostile text and loads the result in the shell. |
| T-K3 | A package overwrites a file of another package. | Only netcheck-owned paths. |
| T-K4 | The man page drifts from the flags. | A test compares the page with the parser; `make man-check`. |

## 5. Tests first

- **CX-1** Each script lists every option, long and short, the `--format` choices, and completes files for bundles and `--template`.
- **CX-2** `completion` with an unknown shell, or none, exits 1 with the usage.
- **CX-3** The bash script loads in a real bash and `compgen` offers `--format` for `--fo`; zsh and fish load when installed (skipped otherwise).
- **CX-4** The man page documents every option the parser has and the three exit codes; `groff` renders it without warnings (skipped if absent).
- **CX-5** The escaping function is fuzzed.
- **CX-6** Lab: the installed `.deb` has the page and the scripts, readable by an unprivileged user, bash sources the script and completes, `man -w netcheck` finds the page.

## 6. Chunks

| Chunk | Content |
| :-- | :-- |
| 0 | This spec and the backlog. |
| 1 | CX-1 to CX-3, CX-5 and the `completion` command. |
| 2 | The man page, `make man`, `make man-check`, CX-4. |
| 3 | goreleaser and nfpm, `make snapshot`. |
| 4 | The manual (chapters 02 and 06), the README. |
| 5 | CX-6 in the lab. |

## 7. Out of scope

A `man` command; completion of values read from a bundle; dynamic completion that runs netcheck; Elvish, Nushell and other shells.
