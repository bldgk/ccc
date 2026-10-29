# ccc — discovery

What had to be settled before `ccc` is designed: what existing tools do, which language it is
written in, and which questions only running code can answer. The requirements are in
[00-requirements.md](../spec/00-requirements.md).

## Existing solutions

The survey is [existing-cleaners.md](tool-caches/existing-cleaners.md). Its result is written
into the requirements, section "Why existing tools are not enough": no surveyed tool describes
a developer tool as user-editable data covering detection, space check and clean with the
tool's own command.

The catalog of 142 tool descriptions the shipped registry is built from is in
[tool-caches/](tool-caches/javascript.md): JavaScript, Python and ML, compiled languages, JVM
and mobile and Apple, containers and infrastructure. 127 of them contain at least one field
marked `unverified`.

## Stack decision

**`ccc` is written in Go.**

The decision was made on what the program itself does, compared against Rust:

| What `ccc` needs | Go | Rust |
|---|---|---|
| A terminal that is fun to look at and use | The Charm libraries are one family in one style: Bubble Tea (screen), Lip Gloss (styling), Bubbles (list, spinner, progress components), Huh (forms) | `ratatui` draws anything, but a selectable list, progress and forms in one style are assembled by hand |
| The three-step flow of adding a tool | A wizard form, which is what Huh is for | A separate prompt library with a different look from the main screen |
| `ccc check`: dozens of tool commands run in parallel, the screen updates as answers arrive, a hung command is cut off by a timeout | Goroutines and Bubble Tea's command model | Threads or async plus channels into the draw loop; possible, more code |
| One binary for macOS and for Linux | Cross-compilation by environment variable, static binary by default, GoReleaser for releases and packages | Building for Linux on macOS needs a separate linker, a static binary needs a separate target |
| Measuring large directories | Fast enough: `gdu` is written in Go | Somewhat faster on a parallel walk; not the bottleneck |
| Parsing registry entries | Variants of an entry are validated by hand-written code | Enumerations validated while parsing; the one place Rust is more convenient |
| Running commands, parsing output, removing directories | Same | Same |

C was not considered further: parsing the registry, running commands and the terminal
interface would all be written by hand with nothing gained.

**The registry is written in TOML**, one file per tool, each place under its own `[[places]]`
header with dotted keys. The reasons are under [The registry file format](#the-registry-file-format).

## Spikes

Two questions could not be settled on paper. Both spikes are done.

### The `ccc check` screen

The prototype was a throwaway Go module, since removed. It detected and measured and
contained no code that removes anything.

Result: the Charm libraries do what the comparison table above assumes.

- Versions used: Bubble Tea v2.1.0, Lip Gloss v2.0.6, Bubbles v2.2.1. The v2 modules are
  published under `charm.land`.
- Out of the box: rows updating live while measurements run in parallel (goroutines write
  results into a channel that the screen reads), the spinner, gradients, borders, `NO_COLOR`.
- Written by hand: the timeout that cuts off a hung command, and the list itself. Rows have
  five states, group headers and size bars, which the ready-made list component does not cover.
- Limits: the palette is tuned for dark backgrounds; the Linux default paths are coded and
  were not run; nobody has looked at the screen in a real interactive terminal yet.

A detection command can change the system: `bun pm cache`, run outside a project, fails and
creates an empty cache directory. A command used for detection in a shipped entry has to be
checked for side effects.

Still open after this spike: what "fun" means for the terminal. The prototype's look is
material for the owner's reaction, not a decision.

### The registry file format

The same three entries — Go, Docker, rustup — are written in TOML and in YAML in
[registry-format/](registry-format/README.md), with the comparison. Both sets parse and are
equal as data.

Result: TOML. Every way found to damage a TOML entry ends in a parse error or in an unknown
key the loader can reject. YAML has damage that loads as a valid entry: a forgotten `- `
overwrites the previous place, ` #` inside a command cuts it off as a comment, a mis-indented
`clean:` lands inside `measure`. For a program that removes files according to these entries,
a damaged entry that loads silently is the worst outcome. The loader rejects unknown keys.

The cost: every command and pattern in TOML needs a choice between `"..."` and `'...'`, and a
regular expression typed in `"..."` does not load. That is a parse error, not silent damage.
The typo cases were run through Python parsers; no Go parser was run.

Writing the three entries showed six things the contract of an entry did not say. Three are
decided in the requirements since: a default path may refer to an environment variable
(`XDG_CACHE_HOME`, `GOPATH`), the name of one installed version gets into a command or a path
through a placeholder, and a command of an entry does not run through a shell. Three are still
open points there:

- a path that lies inside what the tool prints (`rustup show home` prints the parent of the
  toolchains directory);
- the detection results of a place that is not a directory;
- a precondition for cleaning, not only for measuring (a Docker prune needs the daemon).

## Not checked here

The descriptions of Linux system package managers (apt, dnf, pacman, snap, flatpak) are
`unverified` in full, and for many other tools the Linux path is the unverified field: the
research ran on a macOS machine. Detection and the space check of a Linux entry can be checked
only on a Linux machine.
