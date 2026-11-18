# netcheck: backlog

Work agreed for later, with what is known so far. An item here is not a specification: each needs its own, approved, before any code (AGENTS.md rule 1). Update this file when an item is added, started or done.

| No. | Item | Recorded | Status |
| :-- | :-- | :-- | :-- |
| 1 | A man page and shell completions (bash, zsh, fish, PowerShell), before v2.0.0 | 2026-11-18 | Done on `feature/cli-extras` (PR against `feature/lab`): `.specs/netcheck-cli-extras.md` |
| 2 | One clean overnight soak (6 h) before the v2.0.0 tag | 2026-11-18 | Done: the 6 h run's only failures were the lab's DTLS server (F-4), fixed; `lab.sh dtls-stress` passes 150 cycles and 6,000 rounds (`.specs/netcheck-lab.md` §6) |
| 3 | `ligo` depends on netcheck pinned to a commit; bump to v2.0.0 after the release | 2026-11-18 | Waits for the tag (`ligo/.specs/ligo-network-checks.md`) |
