# Registry file format: TOML or YAML

A design spike for the open point "the format of the registry file" in
[00-requirements.md](../../spec/00-requirements.md#open), as described in
[01-discovery.md](../01-discovery.md#the-registry-file-format). Three real entries are written
in both formats with the same field names, so the files differ only in format.

| Tool | TOML | YAML | What it exercises |
|---|---|---|---|
| Go | [toml/go.toml](toml/go.toml) | [yaml/go.yaml](yaml/go.yaml) | two directory places; path from a command, a variable, a per-OS default; disk usage; the tool's own clean command |
| Docker | [toml/docker.toml](toml/docker.toml) | [yaml/docker.yaml](yaml/docker.yaml) | places that are not directories; a size parsed out of command output; a precondition |
| rustup | [toml/rustup.toml](toml/rustup.toml) | [yaml/rustup.yaml](yaml/rustup.yaml) | installed versions: listing, measuring one, removing one |

There is no Go code and no parser here. The field names and their meaning below are what the
three entries needed; they are a proposal for the owner to react to, not a decided schema.

## Validation

- All three TOML files parse with Python 3.13 `tomllib` (TOML 1.0).
- All three YAML files parse with PyYAML 6.0.3 `safe_load` (a YAML 1.1 parser).
- For each tool the parsed TOML and the parsed YAML are equal as data.
- Every "what a typo does" statement below was produced by feeding the mutated text to these
  two parsers. Statements about Go libraries are marked as not checked: no Go parser was run.

## Where the commands and paths come from

Everything is taken from the catalog in `docs/research/tool-caches/` or from the tools'
official documentation. Read-only commands were run to confirm output shapes: `go env`,
`rustup show home`, `rustup toolchain list` with `--verbose` and `--quiet`,
`docker system df` with `--format`, and `--help` of the Docker prune commands. No clean, prune
or uninstall command was run.

Fields that could not be confirmed are marked with a comment in the files:

- the literal Linux default of the Go build cache (derived from the documented rule, not seen
  on a Linux machine);
- the exit status of `docker info` when the daemon cannot be reached (the documentation page
  does not state it; it was only observed with a running daemon);
- what `rustup toolchain list --quiet` prints when no toolchain is installed.

## Assumptions the entries rely on

These are not format questions, but the entries cannot be read without them.

- A command is one string. `ccc` splits it into words the way a shell does (quotes group
  words) and runs it without a shell. `'{{.Type}} {{.Size}}'` in the Docker entry is one
  argument.
- A pattern is a Go regular expression applied to each line of the command's output. The first
  capture group of the first matching line is the value.
- A captured size is a number with a unit as the tool prints it (`0B`, `1.2GB`); `ccc`
  converts it.
- `<path>` and `<version>` in a command or a path are replaced by the resolved path of the
  place and by one listed version.
- `~` at the start of a default path is the user's home directory.

## The field set

| Field | Meaning | Step of the contract | Needed by |
|---|---|---|---|
| `kind` | `tool` or `folder` | the registry: an entry has a kind | all three |
| `name` | the name of the entry shown to the user | the registry | all three |
| `executable` | the program looked up on `PATH` | Detection: "its executable is on `PATH`" | all three |
| `places` | the list of places | Detection: "an entry lists places" | all three |
| `places.name` | the name of the place, one row or one group of rows | Detection | all three |
| `places.holds` | `cache` or `versions` | Detection: "declared either as a cache or as installed versions" | all three; `versions` only in rustup |
| `places.path` | present for a directory place, absent for a place that is not a directory | Detection: "a place is either a directory, or a part of the tool's storage" | Go, rustup |
| `places.path.command` | a command of the tool that prints the path | Detection: first in the resolution order | Go, rustup |
| `places.path.env` | an environment variable that holds the path | Detection: second in the resolution order | Go, rustup |
| `places.path.default.macos`, `.linux` | the default path per operating system | Detection: third in the resolution order | Go, rustup |
| `places.path.subdir` | a directory name appended to what `command` and `env` give | not in the contract, see [Missing from the contract](#missing-from-the-contract) | rustup |
| `places.list.command` | a command that prints the installed versions | Detection: "the entry says how the versions are listed" | rustup |
| `places.list.pattern` | takes one version name out of one output line | Detection | rustup |
| `places.measure.by` | `disk-usage`: allocated blocks of a path | Space check: "the disk usage of the resolved path" | Go, rustup |
| `places.measure.path` | the path measured when it is not the place's own path; holds `<version>` | Space check applied "to one version at a time" | rustup |
| `places.measure.command` | a command of the tool whose output holds the size | Space check: "a command of the tool whose output is parsed" | Docker |
| `places.measure.pattern` | takes the size out of the output | Space check | Docker |
| `places.measure.requires` | a command that must succeed, otherwise the place is "not measured" | Space check: "when measuring needs something that is absent" | Docker |
| `places.clean.command` | the tool's own clean command; holds `<version>` for a place of versions | Clean | all three |

Eighteen fields plus `subdir`. A Go-like place is seven lines of data. Not written because none
of the three entries needs them: removal of the directory as the clean method, and a step
before cleaning.

`measure.by` has a single value in these files. It could be dropped and inferred from the
absence of `measure.command`; it is kept so that a place says how it is measured rather than
saying nothing.

## The open points, written as data

Observations for the owner. Each says what the entry had to contain; none is a decision.

**Parsing a tool's output into a size.** In the Docker entry the user types two lines: a
command and a pattern ([toml/docker.toml:22-23](toml/docker.toml#L22-L23),
[yaml/docker.yaml:23-24](yaml/docker.yaml#L23-L24)). The command uses Docker's own `--format`
template to reduce the output to `<type> <size>` lines, and the pattern picks the line and
captures the size. So the user has to know regular expressions and has to look at the tool's
output once. Two other forms were written and set aside:

- A pattern over the default table (`^Build Cache\s+\d+\s+\d+\s+(\S+)`): no template to learn,
  but a longer pattern that depends on the column order.
- No pattern, the tool prints the single value:
  `docker system df --format '{{if eq .Type "Build Cache"}}{{.Size}}{{end}}'`. It works only
  for tools that have a template language, and it puts double quotes inside single quotes,
  which is the hardest string of the spike for TOML (see [Quoting](#quoting-and-escaping)).

A third form, selecting a key from JSON output (`docker system df --format json` prints one
object per line with the keys `Type` and `Size`), needs a selector language in `ccc` and was
not written. What the pattern form leaves unsaid is the unit: Docker prints decimal units
(`GB`), and a tool that prints a bare number would need a field naming the unit.

**Measuring one installed version.** Written as the disk usage of a path built from the place
and the listed name: `measure.path = "<path>/<version>"`
([toml/rustup.toml:24-25](toml/rustup.toml#L24-L25),
[yaml/rustup.yaml:27-29](yaml/rustup.yaml#L27-L29)). It holds for rustup because a toolchain is
a directory of that name. The alternative is to let the list command give the path:
`rustup toolchain list --verbose` prints `<name> (active, default) <path>` per line, and a
pattern with two groups would yield both. That needs no `path` and no `subdir`, but then a
rustup directory left behind after rustup is removed is never detected.

**The tool's path command exists but fails.** Nothing in an entry expresses this in either
format: if the rule is the same for every tool it is not data. The rustup entry shows one case
where falling through is the only way the contract's "a cache often outlives the tool" can
work: with rustup removed, `rustup show home` cannot run, and the place is found only through
`RUSTUP_HOME` or the default.

**A step that must run before cleaning.** None of the three tools needs one. As data it would
be one more key next to the clean command, `clean.before = "sccache --stop-server"` in TOML or
`before: sccache --stop-server` under `clean:` in YAML; both parse. Several steps would be a
list of strings, which both formats write without trouble.

**The daemon precondition.** `measure.requires` is repeated in both Docker places
([toml/docker.toml:19](toml/docker.toml#L19) and [:31](toml/docker.toml#L31)), and the measure
command is repeated too. The contract speaks of a precondition per place; the file shows that
for Docker it is a fact about the tool. Cleaning needs the daemon as well, and the entry does
not say so.

**Which number a Docker row shows.** It is one word in the entry: `.Size` in the template.
Changing it to `.Reclaimable` also requires changing the pattern, because that column prints
`1.2GB (60%)` for images and `1.2GB` for the build cache.

**Whether Docker images are cleaned by "clean everything".** The images place is declared
`holds = "cache"` ([toml/docker.toml:30](toml/docker.toml#L30)), which under the current
contract puts it in "clean everything". If the answer is no, `holds` needs a third value or the
place needs another field; the two values the contract gives cannot express it.

**A place whose tool was uninstalled.** The Go entry's clean commands cannot run without `go`,
and the fallback the contract names, removal of the directory, fails on the module cache
because it is read-only ([toml/go.toml:32-34](toml/go.toml#L32-L34)). The entry as written has
one clean method per place.

**The command that adds an entry.** With these fields, adding a Go-like place is seven answers:
name, cache or versions, path command, variable, two defaults, clean command. Adding a
Docker-like place is five, and two of them are a template and a regular expression.

## Missing from the contract

Things an entry could not be written without, or could not express.

1. **A path that is inside what the tool reports.** `rustup show home` and `RUSTUP_HOME` give
   the rustup home, while the place is the `toolchains` directory inside it, and the contract
   forbids using the directory above. The rustup entry adds `path.subdir` for this
   ([toml/rustup.toml:15-19](toml/rustup.toml#L15-L19)). It is the only field in the files that
   the contract does not call for.
2. **A default path that depends on an environment variable.** The Linux default of the Go
   build cache is `$XDG_CACHE_HOME/go-build`, and `~/.cache/go-build` only when that variable
   is unset; the module cache default is `$GOPATH/pkg/mod`. The contract allows one path per
   operating system. Nothing was added: the entries hold the literal path and say so in a
   comment ([toml/go.toml:15-19](toml/go.toml#L15-L19)). The literal is wrong exactly when the
   tool is gone and the variable is set.
3. **How one version gets into a command or a path.** The contract says the space check and
   the clean step apply to one version at a time, but not how. The rustup entry uses the
   placeholders `<version>` and `<path>`.
4. **Detection of a place that is not a directory.** The contract's three results are defined
   through a path. For Docker's build cache the entry has nothing to say about "found" or "not
   present on this machine"; nothing was added.
5. **A precondition for cleaning.** The contract names a precondition only under the space
   check. `docker builder prune` needs the daemon just like `docker system df`. Nothing was
   added.
6. **How a command string is run.** Whether a command goes through a shell decides what the
   user may type (pipes, `$(...)`, quotes). The files assume no shell.

## TOML and YAML on these six files

### Size and shape

| | TOML | YAML |
|---|---|---|
| Go | 34 lines | 41 lines |
| Docker | 35 lines | 38 lines |
| rustup | 26 lines | 31 lines |
| deepest nesting | none: every line starts in column 1 | 4 levels, 8 spaces ([yaml/go.yaml:16](yaml/go.yaml#L16)) |
| quote characters around values | every value | none in the three files |

### Nesting

The contract nests four deep: entry, place, path, default, operating system.

YAML shows it as indentation. A place reads as an outline, and the three steps of the contract
are visible as three headings: `path:`, `measure:`, `clean:`
([yaml/go.yaml:10-25](yaml/go.yaml#L10-L25)). The cost is seven lines that carry only a key and
a colon in the Go file, and the place of a line in the hierarchy is known only from the lines
above it.

TOML's usual way of nesting, a table header per level, was tried first and was the most awkward
thing in the spike. A Go place became `[[places]]`, `[places.path]`, `[places.path.default]`,
`[places.measure]`, `[places.clean]`: five headers for seven values, each repeating the word
`places`, with two real traps described under [typos](#what-a-typo-does). The files use dotted
keys instead: one `[[places]]` header and then full key paths
([toml/go.toml:9-21](toml/go.toml#L9-L21)). Each line states its whole position,
`path.default.linux = ...`, at the price of repeating `path.` five times and
`measure.` three times in one place ([toml/docker.toml:19-23](toml/docker.toml#L19-L23)).
Inline tables were also tried for `path`; TOML 1.0 does not let an inline table span lines, and
the one-line form ran past 120 characters.

Where YAML is better: a field that became a list inside a place (several steps before
cleaning, several candidate default paths) stays an indented list. In TOML it is an array on
one line or a second level of `[[places.x]]` headers, and dotted keys cannot be mixed into it.

Where TOML is better: comments attach to exactly one line. In YAML the comment about the Linux
default sits four levels deep ([yaml/go.yaml:17-20](yaml/go.yaml#L17-L20)) and must be indented
to match by convention, and the comment about `clean` had to move above the `clean:` heading,
one line away from the command it explains ([yaml/go.yaml:38-41](yaml/go.yaml#L38-L41) against
[toml/go.toml:32-34](toml/go.toml#L32-L34)).

`[[places]]` reads slightly wrong: TOML convention is a singular name for an array of tables.
The plural was kept because it is the right name in YAML and the names had to match.

### Quoting and escaping

The requirement is "hold commands and patterns without escaping".

YAML: none of the six commands and three patterns needed quotes or escapes. The Docker command
with single quotes and braces and the pattern with a backslash are written as they would be
typed in a terminal ([yaml/docker.yaml:23-24](yaml/docker.yaml#L23-L24)). The template with
double quotes inside single quotes also parses unquoted. This is the best result of either
format on these files.

It holds only while the value does not hit one of YAML's rules, and each of these was tried:

| A value a user could plausibly type | Result |
|---|---|
| `pattern: [0-9]` | no error: the value is a list with one element `"0-9"` |
| `pattern: [0-9.]+GB` | parse error |
| `format: {{.Size}}` (a value starting with a brace) | parse error |
| `pattern: *.tmp` | parse error ("alias") |
| `command: docker system df --format '{{.Type}}: {{.Size}}'` (colon and space) | parse error |
| `pattern: ^Build Cache:` | parse error |
| `command: grep -v ^ # comments` (space and hash) | no error: the value is `grep -v ^` |
| `pattern: "^Images (\S+)$"` (the user adds double quotes to be safe) | parse error; single quotes are the safe ones |
| `name: no`, `holds: on`, `version: 1.80`, `default: ~` | no error: `false`, `true`, the number `1.8`, null (YAML 1.1 parser) |

This is why the placeholders are `<version>` and not `{version}`: `path: {path}/{version}`
is a parse error in YAML, and braces also collide with Docker's templates. The choice of
placeholder was bent by the format.

TOML: every value is quoted, and there are two kinds of quotes with different rules, which the
user has to choose between:

- `"..."` for commands, because the Docker command contains single quotes
  ([toml/docker.toml:22](toml/docker.toml#L22));
- `'...'` for patterns, because a backslash in `"..."` is an escape
  ([toml/docker.toml:23](toml/docker.toml#L23)). `pattern = "^Images (\S+)$"` is a parse error.

A string with both a single quote and a backslash fits neither and needs `'''...'''`. The
template with inner double quotes is either `"...\"Build Cache\"..."`, which is escaping, or a
`'''` string that ends in four quote characters. Neither of the three shipped entries needs
this, but it is one template away. One silent case was found: `"\t"` inside a `"..."` command
becomes a real tab character rather than the two characters Docker's template expects.

On the requirement taken literally, YAML does better on these six files: zero quotes against
quotes everywhere and one mandatory choice of quote kind.

### What a typo does

All results from the two parsers named under [Validation](#validation).

| Typo | TOML | YAML |
|---|---|---|
| quotes forgotten or unbalanced | parse error | not applicable; see the table above |
| the same key twice in one place | parse error | no error, the last one wins (PyYAML; a Go parser was not checked) |
| the start of the second place forgotten (`[[places]]` / the `- `) | parse error: the keys repeat | no error: one place, the second overwrites the first, the first place is gone |
| a line indented one level too deep (`clean:` under `measure:`) | not applicable | no error: `clean` becomes a field of `measure`, the place has no clean step |
| a line indented one level too shallow (`env:` out of `path:`) | not applicable | no error: `env` becomes a field of the place |
| indentation off by one space, a tab | not applicable | parse error |
| a misspelt key (`mesure.path`) | no error: an unknown key | no error: an unknown key |
| a top-level key written after `[[places]]` | no error: `executable` becomes a field of the last place | not applicable while indentation is right |
| `[places]` instead of `[[places]]` | no error with one place (a table, not a list); parse error with two | not applicable |
| table-header style: a key added at the end of a place | no error: it lands in the last sub-table (`holds` inside `clean`) | not applicable |
| table-header style: `[clean]` instead of `[places.clean]` | no error: a top-level `clean` table | not applicable |

Reading of the table: in TOML with dotted keys the remaining silent cases all produce a key in
the wrong table or a wrong type, which a loader that rejects unknown keys and checks types
turns into an error. In YAML three silent cases produce data that is valid against any schema:
a truncated command, `[0-9]` as a list where a string was meant (caught by a type check), and
above all a lost place, where the file loads as a well-formed entry with one place fewer. The
last two rows are the reason the files do not use table headers.

Both formats need the loader to reject unknown keys; neither helps with a misspelt key by
itself.

### A contributor's one-line change

Changing the Linux default of the Go build cache:

```diff
--- toml/go.toml
-path.default.linux = "~/.cache/go-build"
+path.default.linux = "~/.cache/golang-build"

--- yaml/go.yaml
-        linux: ~/.cache/go-build
+        linux: ~/.cache/golang-build
```

The TOML line names the field in full; the reviewer still needs the hunk context to see which
place it is in, since both Go places have this key. The YAML line needs the context for both
the field and the place: `linux:` under which `default:` under which `path:`.

Adding a field to an existing place, for example a step before cleaning:

```diff
--- toml
+clean.before = "sccache --stop-server"

--- yaml
     clean:
+      before: sccache --stop-server
       command: ...
```

In TOML the line can go anywhere inside the place and means the same. In YAML it must go under
the right heading at the right depth, and the two wrong depths that are still valid YAML put it
somewhere else without an error.

Adding a place: in both formats it is a block copied from another place. In YAML forgetting the
`- ` loses the previous place silently; in TOML forgetting `[[places]]` is a parse error.

### What turned out awkward

- TOML table headers per nesting level: verbose and with two silent traps. Replaced by dotted
  keys.
- TOML inline tables for `path`: must fit on one line.
- Curly-brace placeholders: a parse error in YAML at the start of a value, and a collision
  with Docker templates in both formats. Replaced by `<version>`.
- A `: ` separator in the Docker template (`'{{.Type}}: {{.Size}}'`): the first form written,
  a parse error in YAML unless the whole command is quoted. Replaced by a space, which means
  the YAML file's lack of quotes is partly the result of avoiding what would need them.
- A command as a list of arguments instead of a string: removes the question of word
  splitting, but nobody can paste it into a terminal, and in YAML the flow form
  `[docker, system, df, --format, {{.Type}} {{.Size}}]` is a parse error.
- Places as a table keyed by name (`[places."build cache"]`, `places:` with `build cache:`
  below): shorter headers in TOML, but a name with a space needs a quoted key, and TOML does
  not promise the order of tables.
- The same field names in both formats: `places` is natural in YAML and unidiomatic as
  `[[places]]`.

## Recommendation

TOML, written with dotted keys under one `[[places]]` header per place, and a loader that
rejects unknown keys.

The reason: the registry is a file of commands and patterns that users edit by hand, and on
these entries every way found to damage a TOML file ends in a parse error or in a key the
loader can reject, while YAML has ways to damage it that load as a valid entry: a place
overwritten by the next one, a command cut at ` #`, a clean step moved inside `measure`. For a
program whose commands delete things, an entry that loads but means something else is the
worse failure.

The single strongest argument against it: the requirement says the format must hold commands
and patterns without escaping, and on these six files YAML meets it with no quotes at all,
while TOML makes the user pick between `"..."` and `'...'` on every command and pattern, and
a pattern typed in the wrong kind does not load. A user adding their first entry meets that
rule immediately; they may never meet any of YAML's traps.

Not checked in this spike and relevant to the choice: how the Go libraries for each format
behave on duplicate keys and unknown keys, and whether a file can be rewritten by the command
that adds an entry without losing its comments.
