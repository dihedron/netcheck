# Overview

`netcheck` is one library and one command that uses it. This manual explains how the pieces fit, why they are shaped as they are, which threats were considered and how the result is tested. Chapters 1 to 8 are for whoever installs and runs the command, chapters 9 to 13 for whoever reviews or maintains it, chapters 14 to 17 for programmers who use the library.

![From the command line to the printed result](architecture.pdf){width=100%}

## The pieces

| Piece | Role |
| ------------------- | -------------------------------------------------------------------------- |
| Command | Parses the command line, loads each bundle, runs it, prints the result |
| Bundle sources | Fetch the text of a bundle from a file, an HTTP(S) server, Redis or Consul and tell its format |
| Bundle | The parsed document: identity, defaults (timeout, retries, wait, concurrency) and the list of checks |
| Library | `pkg/netcheck`: the worker pool (at most `concurrency` checks at a time) and one runner per protocol, each attempt bounded by the timeout, failures retried; `pkg/netcheck/fetch` and its sub-packages load bundles; `dtls` and `sso` plug in the heavy protocols. The command is a thin user of it (chapter 10) |
| Output | Text, JSON, YAML or a user template, with colours where the terminal supports them |

## A run, step by step

1. The command receives zero or more bundle locations. With none, it runs on mock data, which is meant for developing templates (chapter 6).
2. For each location, in order, the matching **source** fetches the text. The scheme of the location picks the source (chapter 6); anything that is not a recognised URL is a file name.
3. The **format** is hinted by the source: from the file extension or from the `Content-Type` header of an HTTP response; without a hint the content decides (a document starting with `---` is YAML, one starting with `{` or `[` is JSON, anything else is tried as JSON and then as YAML).
4. The bundle is parsed. Values the bundle does not set come from the defaults (chapter 7).
5. The checks are queued for a pool of `concurrency` workers. Each worker takes a check, runs it with its timeout, and on failure waits and retries, up to `retries` attempts in total.
6. When all checks of the bundle are done, the results are printed (text format) or kept until every bundle has run (JSON, YAML, template), so that the output is one valid document.

## Two ways in

The command is for people and scripts: it reads the machine's configuration (the defaults file, the environment), parses a command line, draws a spinner, prints text and sets an exit status. The library is for programs: it reads nothing, prints nothing, keeps no state and never exits the process; everything it needs is passed in, and everything it found is returned. The split is the main design decision: every behaviour that depends on the machine or the terminal lives in `cmd/netcheck` and `internal/`, and everything that decides whether a connection works lives in `pkg/`.
