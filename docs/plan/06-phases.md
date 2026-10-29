# ccc — phases

The order in which `ccc` is built. This file holds order, dependencies and status only; what
is built is decided in the spec, [docs/spec/INDEX.md](../spec/INDEX.md).

The order runs from the core outwards: the engine that detects, measures and cleans comes
first, the surfaces that operate it after, the breadth of the shipped registry last.

| Phase | What it builds | Implements | Depends on | Status |
|---|---|---|---|---|
| 1. Core | `internal/entry`, `internal/registry`, `internal/sys` with `host` and `fake`, `internal/engine`, and their tests | [Architecture](../spec/05-architecture.md): all sections | — | done |
| 2. First shipped entries | `registry/*.toml` for the tools whose detection and space check can be confirmed on a macOS machine | [Requirements](../spec/00-requirements.md), "What the shipped registry contains"; [Architecture](../spec/05-architecture.md#the-entry-file) | the entry schema of phase 1 | done: 11 entries |
| 3. `ccc -c` | `cmd/ccc` and the output of cleaning everything | [Screens](../spec/04-screens.md) | 1, 2 | done; run on test data only, not on the shipped entries |
| 4. The screen | `internal/ui`: the `ccc check` screen, confirmation, result | [Screens](../spec/04-screens.md) | 1, 2 | done; of the shipped entries, only the pip cache was cleaned on a real machine |
| 5. Adding entries | The command that adds a tool or a folder | [Requirements](../spec/00-requirements.md), "Add a tool", "Add a folder" | 1 | done |
| 6. Linux and the rest of the catalog | Confirming entries on Linux, adding entries from the catalog one by one as each is confirmed | [Requirements](../spec/00-requirements.md), "What the shipped registry contains" | 2 | not started |
| 7. Release | Building and publishing the binaries | — | 3, 4, 5 | not started |

Phases 1 and 2 touch different directories and run at the same time.
