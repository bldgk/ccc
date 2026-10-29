# Existing developer-cache cleaners: prior-art survey for `ccc`

Method: GitHub API (stars, `pushed_at`, latest release), project READMEs and source files, and project web pages. Nothing was installed or run. "unverified" marks anything not confirmed from a primary source.

## Summary

Several existing tools overlap with parts of this idea, and one overlaps strongly. The relevant tools fall into three groups.

1. **Data-driven cleaner registries.** BleachBit with its CleanerML XML format is the only mature one. It is extensible by users, but it targets paths (plus a closed list of coded commands) and mostly covers desktop apps, not developer tools.
2. **Developer-cache cleaners with a hard-coded tool list.** These are mac-cleanup-py, Mole, CleanMyMac CLI, sasurahime, broom, devclean (ohing504), cacheout and others. They scan, show sizes, let the user pick, and often call native commands such as `npm cache clean`, `uv cache prune` and `pip cache purge`. Some discover paths by asking the tool (Mole runs `npm config get cache` and `uv cache dir`; mac-cleanup-py runs `brew --cache`). In all of them the tool knowledge is code, not user-extensible data. The exception is mac-cleanup-py, where users can add Python module files.
3. **Per-project artifact cleaners and size analyzers.** kondo, npkill, devclean (HuakunShen), cargo-sweep, cargo-cache, dust, gdu and ncdu. These cover `node_modules`/`target`-style folders or only measure.

One tool, `devspace-sweeper`, uses YAML rule files, but they are glob rules only (no native commands, no path discovery).

No tool was found that combines user-extensible data entries, path discovery by asking the tool, and the tool's native clean command, run on both macOS and Linux. See `## Gaps`.

## Comparison table

Legend: Y = confirmed, N = confirmed absent, partial = see note, unverified = not confirmed from a primary source. Stars and dates are from the GitHub API (`pushed` = last push to the default branch).

| Tool | Platforms | Tool knowledge | Native clean cmd or own delete | Path discovery | Sizes before clean + user pick | Per-project folders | Docker | Activity |
|---|---|---|---|---|---|---|---|---|
| BleachBit | Windows, Linux (repo description); macOS: cleaners exist (`os="macos"` in docker.xml) but README does not state support, so unverified | Data: CleanerML XML, user-extensible (user cleaner dirs) | Mostly own delete/truncate; a closed set of coded commands (`apt.clean`, `yum.clean_all`, `dnf.clean_all`, `pacman.cache`, `snap.disabled`, `journald.clean`); generic `process` action runs an arbitrary command with a `cmd` attribute | Fixed paths with variables/globs/OS conditions; no "ask the tool" | Y (preview of sizes, per-option checkboxes) | Partial: `walk.*` and `deep` searches can match by name/regex, no project-marker logic | Only Docker Desktop app cache/logs/AI-assistant history (docker.xml), no prune | 7,094 stars; push 2026-10-11; v6.0.4 2026-09-08 |
| kondo | Win/macOS/Linux (README topics, package managers) | Hard-coded Rust (`ProjectType` enum + const dir arrays in `kondo-lib/src/lib.rs`) | Own delete ("rm -rf with a prompt") | Project marker files (e.g. `Cargo.toml`) | Y (size shown, prompt) | Y (core purpose; 23 project types) | N | 2,448 stars; push 2026-04-24; v0.9 2026-01-23 |
| npkill | Platforms not listed in README (Node >= 14); unverified | Targets configurable by name (`-t`, profiles, `.npkillrc`) | Own delete | Directory-name search | Y | Y (`node_modules` default, other names via targets) | N | 9,468 stars; push 2026-10-12; v0.12.2 2024-06-08 |
| cargo-cache | Linux example in README; OS list unverified | Hard-coded (cargo home layout) | Own delete in `$CARGO_HOME` (README does not mention calling external commands) | `$CARGO_HOME` | Stats shown; selection via flags, not a picker | N (cargo home only) | N | 994 stars; push 2023-06-04; no GitHub releases |
| cargo-sweep | Cargo subcommand, OS list unverified | Hard-coded | Own delete of old `target/` artifacts (age, toolchain, size limit) | `target/` dirs | Dry-run; flags, no picker | Y (`target/` of many Rust projects) | N | 996 stars; push 2026-05-26; v0.8.0 2025-10-11; README says unmaintained, points to `cargo-clean-all` |
| mac-cleanup-py | macOS only | Python module functions; users can load their own module files (`-p/--custom-path`); not declarative data | Mixed: `Command("npm cache clean --force")` plus `Path(...)` deletes | Partly: `brew --cache` is queried; other modules use fixed paths | Y (dry run, `-c` module config screen) | N | Y (dangling images, stopped containers, via docker CLI) | 2,378 stars; push 2026-04-14; v3.3.0 2025-03-19 |
| mac-cleanup-sh | macOS | Shell script, hard-coded | Mixed (shell) | Fixed | partial (unverified) | N | unverified | 2,815 stars; push 2023-05-21; v1.1.9 2021-08-17 |
| Mole (`mo`) | macOS (Windows on a separate branch) | Hard-coded Bash/Go (`lib/clean/dev.sh`, 5,723 lines) | Both: native (`npm cache clean --force`, `uv cache prune`, `conda clean`, `corepack cache clean`, `bun pm cache rm`, `pnpm store prune`) plus `safe_clean` path delete for residue | Y for some: `npm config get cache`, `uv cache dir`, `bun pm cache` | Y (`--dry-run`, whitelist; `mo purge` selection UI with sizes and age) | Y (`mo purge`: `node_modules`, `target`, `.build`, `build`, `dist`; scan paths configurable) | Docker prune: no hit in `dev.sh` grep (other files unchecked), unverified | 70,058 stars; push 2026-10-12; V1.59.1 2026-10-10; GPL-3.0 |
| CleanMyMac CLI (MacPaw) | macOS 11+ | Closed source (repo has only README, SECURITY, EULA LICENSE); unknown | Unknown (closed) | Unknown | Y (preview, approval, `--force` to skip) | Y (`purge` over `~/Projects`, `~/Code`, `~/dev`, ...) | Y (listed under `clean dev`) | 1,015 stars; push 2026-08-26; v1.0.0 2026-07-17; proprietary EULA; "early developer preview" |
| CleanMyMac (GUI) | macOS only | Closed | Closed | Closed | Y | not mentioned on product page | not mentioned | Commercial; free trial, "starting at $3.33/month" |
| DaisyDisk | macOS 10.13+ | N/A (analyzer) | Own delete (permanent) | N/A | Y (visual map; user picks) | N | N | Commercial, $9.99 lifetime |
| DevCleaner for Xcode (vashpan) | macOS | Hard-coded for Xcode | Own delete (Device Support, Archives, DerivedData, docs cache, old logs) | Fixed Xcode dirs | Y | N | N | 1,616 stars; push 2026-05-18; 2.8.0 2025-11-11; GPL-3.0; Mac App Store |
| Czkawka / Krokiet | Linux, Windows, macOS, others | Hard-coded finders (duplicates, empty files/folders, temporary files, similar images...) | Own delete | N/A | Y | N | N | 34,211 stars; push 2026-09-16; 12.0.2 2026-09-09 |
| dust | Linux, macOS, Windows | N/A (measure only) | None | N/A | Sizes yes, no deletion | N/A | N | 12,494 stars; v1.2.6 2026-09-16 |
| gdu | Linux/macOS/Windows (Windows unverified) | N/A (analyzer; has delete key, `--no-delete` flag) | Own delete on request | N/A | Y | N | N | 6,102 stars; v5.38.0 2026-10-05 |
| ncdu | Any POSIX-like, Windows binary | N/A (analyzer) | Deletion not confirmed from the project page | N/A | Y | N | N | 2.9.2 (2025-10-24) stable, 1.22 (2025-03-05) LTS (project page); GitHub mirror `rofl0r/ncdu` is a fork, last push 2020 |
| pyclean (bittner) | Cross-platform (per description) | Hard-coded (Python bytecode only) | Own delete | N/A | unverified | Y (`__pycache__`, `.pyc`) | N | 92 stars; push 2026-05-23 |
| sasurahime | macOS | Hard-coded Rust (`Cleaner` trait + `define_cleaners!` macro; HOWTO says adding a target needs 3 source edits) | Both (docs: "external CLI command or directory deletion"); Trash by default | Fixed cache dirs (`~/.cache`, `~/Library/Caches`...) | Y (`scan`, interactive menu, `--dry-run`) | partial (mise per-project `.mise.toml` check; no artifact scan found) | Y (listed among 40+ tools) | 5 stars; push 2026-06-23; Apache-2.0 |
| broom (stefanosbou/broom-cli) | macOS, Linux, Windows | Hard-coded Go (`Scanner` interface, `register.go`); `config.toml` holds preferences only | Deletion; whether native commands are used is not stated | Fixed paths; Docker via `docker system df` | Y (picker, `--dry-run`) | partial (project artifacts for some ecosystems; details unverified) | Y (dangling images, build cache) | 0 stars; push 2026-05-14 |
| devclean (ohing504) | macOS/Linux, amd64/arm64 | Hard-coded Go ecosystems; config "not yet implemented" (SPEC) | Both: own delete plus ecosystem-native prune (`npm cache clean --force`, `yarn cache clean`, `pnpm store prune`, `pip cache purge`, `uv cache prune`, `xcrun simctl delete unavailable`), skipped when tool not on PATH | Marker files per project; fixed global paths | Y (scan table, tree selector, `--dry-run`) | Y (node, rust, ruby, python, go, flutter, android) | Scan only; "prune deferred" | 1 star; push 2026-10-02; v0.2.0 2026-10-02 |
| devclean (HuakunShen) | GUI + CLI (platforms unverified) | Hard-coded Rust traits (`LanguagePredicate`, `Removable`, `Stop`, `Reportable`); Node and Rust only | Own delete | Marker files | Y | Y | N | 107 stars; push 2026-09-25 |
| devspace-sweeper | Cross-platform claimed; Windows listed as future | Data: YAML rule file (`recipes/default.yml`, `--recipes` for custom) | Glob-based; README does not say how it cleans | Globs | scan/suggest/clean/report; dry-run claimed | Y (globs like `**/node_modules/**`, `**/target/**`) | N | 0 stars; push 2026-01-17; 0.1.5 on crates.io 2026-01-15 |
| nathankim0/mac-dev-cache-cleaner | macOS | README: "no hardcoded project names"; discovery via marker files | Docker via Colima prune (native) | Marker files | unverified | Y | Y (Colima) | 0 stars; push 2026-08-20 |
| toufiqism/devclean | macOS, Linux, Windows | Hard-coded Python single file | Own delete | XDG dirs honored on Linux | Y (checkbox TUI with SAFE/REVIEW tags) | NDK pinning check; not a project scanner (unverified) | N | 5 stars; push 2026-08-23 |
| cacheout (cacheout-app) | macOS 14+ | Hard-coded Swift | Own delete (Trash option); Docker row says to run `docker system prune -a` first | Fixed + marker files for project artifacts | Y | Y (marker-verified `target/`, `node_modules/`) | Disk image only; prune is the user's job | 2 stars; push 2026-09-06 |
| ylevytskyy/mac-cleaner | macOS | Hard-coded shell | Both ("per-tool prune for Docker / OrbStack / Colima / Lima / Podman", plus fixed dotcache deletes) | Fixed | unverified | N | Y (volumes only with `--include-volumes`) | 0 stars; push 2026-09-16 |

Other very small repositories found by search (0-5 stars, not examined in depth): `gzb1128/mac-dev-cleanup`, `malvernbright/syssweep`, `yogaananda6677/ubuntu-system-cleaner`, `jalioba/GoClean`, `Mahmud0808/DevCleaner`, `NeverMorewd/DevCleaner` (Windows), `dockerase` (crate listing only, unverified).

## BleachBit CleanerML

CleanerML is an XML dialect validated against a schema (the docs say XSD; the repository `bleachbit/cleanerml` carries the spec/pending cleaners). Definitions live in `cleaners/*.xml` in the BleachBit repository (about 100 files at the time of survey, including `docker.xml`, `python.xml`, `vscode.xml`, `claude.xml`, `android_studio.xml`, `apt.xml`, `snap.xml`, `pacman.xml`). Users can drop their own files into `~/.config/bleachbit/cleaners/` (Linux) or `%APPDATA%\BleachBit\cleaners\` (Windows). License: GPL-3.0.

### Elements and attributes

- `<cleaner id=".." os="..">`: one tool. Children: `<label>`, `<description>`, `<running>`, `<var>`, `<option>`.
- `<running type="exe|pathname" os=".." same_user="true|false">name</running>`: abort cleaning if the application is running.
- `<var name="x"><value os="linux|macos|windows">path</value>...</var>`: per-OS path variable, referenced as `$$x$$`. Paths expand `~`, environment variables (`$HOME`, `$XDG_CACHE_HOME`, `%AppData%`).
- `<option id="..">`: one user-selectable checkbox. Children: `<label>`, `<description>`, optional single `<warning>`, one or more `<action>`.
- `<action command=".." search=".." path=".." regex=".." nregex=".." wholeregex=".." nwholeregex=".." type="f|d" os=".." name=".." cmd=".." wait="..">`.
  - `search` (how files are found): `file`, `glob`, `walk.files`, `walk.all`, `walk.top`, `deep`, `children`.
  - `command` (what to do): `delete`, `truncate`, `shred`, `ini`, `json`, `sqlite.vacuum`, `winreg`, `process`, plus coded commands registered in `bleachbit/Action.py` (`apt.clean`, `apt.autoclean`, `apt.autoremove`, `yum.clean_all`, `dnf.clean_all`, `dnf.autoremove`, `pacman.cache`, `snap.disabled`, `journald.clean`, `chrome.*`, `mozilla.*`, `cookie`, `office_registrymodifications`, `win.shell.change.notify`, `macos.orphaned_framework_versions`).
  - `process` (verified in `Action.py`): runs `cmd` (env-expanded, shell-split, waits unless `wait="false"`); the exit code is only logged as a warning and the action returns 0.

### Real example (from `cleaners/docker.xml`, abbreviated)

```xml
<cleaner id="docker">
  <label>Docker Desktop</label>
  <description>Container platform</description>
  <running type="exe" os="macos" same_user="true">Docker Desktop</running>
  <var name="base">
    <value os="linux">$XDG_CONFIG_HOME/Docker Desktop</value>
    <value os="macos">~/Library/Application Support/Docker Desktop</value>
    <value os="windows">%AppData%\Docker Desktop</value>
  </var>
  <option id="cache">
    <label>Cache</label>
    <description>Delete the cache</description>
    <action command="delete" search="walk.all" path="$$base$$/Cache/"/>
    <action command="delete" search="walk.all" path="$$base$$/GPUCache/"/>
    <action command="delete" search="file" path="$$base$$/Network Persistent State" os="unix"/>
  </option>
</cleaner>
```

Native-command example (`cleaners/apt.xml`): `<action command="apt.clean"/>`. That works only because `apt.clean` is a Python class in BleachBit's code, not because the XML names a command line.

### What CleanerML cannot express (verified from the docs, the example file and `Action.py`)

- Running an arbitrary native clean command *and* reading its result: `process` exists but the exit code is discarded, stdout is not captured, and no size is derived from it.
- Discovering a path by asking the tool (for example `npm config get cache`, `uv cache dir`): path values are static strings with variable expansion only.
- Reporting size for a native command: previews come from the file-search commands; a `process` action has no preview size.
- Detecting that a tool is installed other than through file existence or `<running>` (a process check, not an installed check).
- Project-marker logic ("`target/` only beside `Cargo.toml`"): `walk.*` plus `regex`/`type` filters match by name only, with no sibling-file conditions.
- Age, size or recency conditions on what to delete.
- Dependencies between entries, or per-entry parameters set by the user.
- Cross-platform command variants for `process` (the `os` attribute exists on `action`, but the command string is a single literal).
- A new native command without editing Python (the coded command list is closed).

## Reusable datasets and specifications

| Source | What it actually contains | Fit for cache locations |
|---|---|---|
| BleachBit `cleaners/*.xml` (GPL-3.0) | About 100 cleaner definitions with per-OS paths, running-process guards, labels and warnings. Mostly desktop apps; developer-relevant: docker (Desktop app), python (history only), vscode, android_studio, java, apt/dnf/pacman/snap. | Real per-OS path data in a machine-readable format; thin on developer tools; GPL-3.0 licensing. |
| kondo `kondo-lib/src/lib.rs` (MIT) | Compiled-in table: project type -> marker file(s) -> artifact dir names (`Cargo`: `target`, `.xwin-cache`; `Node`: `node_modules`, `.angular`; Maven/SBT: `target`; plus 20 more types). Not an external data file. | Good source for project-marker/artifact pairs. |
| GitHub `gitignore` templates (github/gitignore) | 165 top-level templates plus `Global/` and `community/`; each lists patterns generated by that language's toolchain (Rust.gitignore: `target`, `debug`, `*.rs.bk`, `rustc-ice-*.txt`). | Lists generated or ignorable files, not caches and not safe-to-delete guarantees; includes IDE and source-level ignores. |
| CACHEDIR.TAG spec (bford.info/cachedir) | A tag file `CACHEDIR.TAG` whose first 43 bytes are `Signature: 8a477f597d28d172789f06886806bc55`, placed in the top directory of cache data; honored by `tar --exclude-caches` and some applications. The spec says tags are hints. | Lets a scanner recognise cache directories without a registry; cargo and some other tools write it (this survey did not check which). |
| XDG Base Directory spec | `XDG_CACHE_HOME` default `$HOME/.cache`, `XDG_DATA_HOME` `$HOME/.local/share`, `XDG_CONFIG_HOME` `$HOME/.config`, `XDG_STATE_HOME` `$HOME/.local/state`. No cache cleanup policy and no per-application layout requirement. | Gives the base directory on Linux; says nothing about what is inside or how to clean it. macOS (`~/Library/Caches`) is outside the spec. |
| Mole `lib/clean/dev.sh` (GPL-3.0) | A working table-in-code of tool -> detect command -> cache-dir query -> native clean command -> residue paths (npm, pnpm, yarn, bun, uv, conda, corepack, pip...). | Readable reference for which native commands and queries exist; code, not data. |
| devclean (ohing504) `docs/SPEC.md` (MIT) | Documented ecosystem list, safety levels (`safe`, `protected`...), per-artifact categories, and the native-prune table above. | Useful for risk classification. |
| mac-cleanup-py `default_modules.py` (Apache-2.0) | About 40 Python module functions with exact paths and commands. | Reference for macOS paths. |

I did not find a standalone, tool-neutral dataset (JSON/TOML/YAML) of developer-tool cache locations with native clean commands.

## Gaps

None of the surveyed tools does all of the following. Each item is "not found in the surveyed projects", not a claim about every project on GitHub.

1. **A user-extensible data registry whose entries carry a native clean command.** BleachBit's registry is data but its native commands are a closed, coded set and `process` discards output. The tools that do call native commands (Mole, mac-cleanup-py, devclean, sasurahime, CleanMyMac CLI) keep that knowledge in code, or in a closed binary. mac-cleanup-py's custom modules are Python code, not declarative data.
2. **Path discovery by asking the tool, expressed as data.** Mole and mac-cleanup-py ask the tool (`npm config get cache`, `uv cache dir`, `brew --cache`) but only inside scripts or code; CleanerML has no such construct.
3. **A size measurement defined per entry as data** (measure path vs. ask the tool vs. a tool-specific report such as `docker system df`). Only broom/devclean mention Docker size via the Docker CLI, in code.
4. **A user-defined entry that is a plain folder removed with `rm -rf`, added through a command and a TOML file.** BleachBit takes hand-written XML in a config directory; npkill takes target names in `.npkillrc`; devspace-sweeper takes a YAML rule file. None of them offers an add-entry command; no TOML-based registry was found.
5. **Linux and macOS together with developer-tool coverage.** Mole, mac-cleanup, sasurahime, CleanMyMac CLI, cacheout, DevCleaner for Xcode are macOS-only; BleachBit covers Linux but has little developer-tool content; broom and devclean (ohing504) cover both but are code-defined and have 0-1 stars.
6. **Docker cleaning through the native command as a selectable entry with a measured size.** mac-cleanup-py and ylevytskyy/mac-cleaner prune; broom scans via `docker system df`; ohing504 devclean's Docker prune is documented as deferred; BleachBit handles only the Docker Desktop app cache.
7. **A single registry covering both global caches and per-project folders.** kondo, npkill, cargo-sweep and devclean (HuakunShen) handle per-project folders only; Mole and ohing504 devclean combine both but as code.
8. **An open, extensible definition set where built-ins and user entries are the same kind of object.** In BleachBit user cleaners share the format with built-ins (the closest match); elsewhere built-ins are code and user additions (when possible at all) are a different mechanism.
9. **Community or standard dataset of tool cache locations.** None found (see the datasets section).

## Sources

- https://docs.bleachbit.org/doc/cleanerml.html
- https://github.com/bleachbit/bleachbit (and `doc/example_cleaner.xml`, `cleaners/docker.xml`, `cleaners/apt.xml`, `cleaners/python.xml`, `bleachbit/Action.py`)
- https://github.com/bleachbit/cleanerml
- https://github.com/tbillington/kondo
- https://github.com/voidcosmos/npkill
- https://github.com/matthiaskrgr/cargo-cache
- https://github.com/holmgr/cargo-sweep
- https://github.com/dnlmlr/cargo-clean-all
- https://github.com/mac-cleanup/mac-cleanup-py
- https://github.com/mac-cleanup/mac-cleanup-sh
- https://github.com/tw93/Mole
- https://github.com/MacPaw/cleanmymac-cli
- https://macpaw.com/news/cleanmymac-cli
- https://macpaw.com/cleanmymac
- https://daisydiskapp.com/
- https://github.com/vashpan/xcode-dev-cleaner
- https://github.com/qarmin/czkawka
- https://github.com/bootandy/dust
- https://github.com/dundee/gdu
- https://dev.yorhel.nl/ncdu
- https://github.com/bittner/pyclean
- https://github.com/armaniacs/sasurahime
- https://pkg.go.dev/github.com/stefanosbou/broom-cli
- https://github.com/ohing504/devclean
- https://github.com/HuakunShen/devclean
- https://github.com/adwaitdeshpande/devspace-sweeper and https://docs.rs/devspace-sweeper
- https://github.com/nathankim0/mac-dev-cache-cleaner
- https://github.com/toufiqism/devclean
- https://github.com/cacheout-app/cacheout
- https://github.com/ylevytskyy/mac-cleaner
- https://github.com/github/gitignore
- https://bford.info/cachedir/
- http://specifications.freedesktop.org/basedir/latest/
