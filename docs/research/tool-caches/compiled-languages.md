# Compiled-language toolchains and compiler caches: verified facts for the `ccc` registry

Summary. This file records, for 27 tool entries (Rust, Go, C/C++ build systems and caches, Zig, Nim, Haskell, OCaml, Elixir/Erlang, Crystal, D), how to detect each tool, where its caches live, how to discover that path, how to measure and clean it with the tool's own command, whether it is regenerable, and what breaks if it is cleaned while in use. Every field carries a verification tag: `official-docs` (read in the official documentation during this research), `ran-command` (confirmed by running a read-only command), or `unverified` (from memory or inferred; do not ship without checking). Several documentation pages were truncated or lacked detail, so many Conan, vcpkg, Zig, Haskell, Elixir and D facts are `unverified`. The critical non-default case is Cargo's `build.target-dir` (settable in `~/.cargo/config.toml`, which makes all projects share one directory); section "cargo (build output / target)" documents how to discover it.

Verification legend per entry: `V(discover / default-paths / clean)`.

---

## cargo (CARGO_HOME: registry, git checkouts)
- `name`: Cargo home cache; `ecosystem`: Rust
- `detect`: `cargo` on PATH, or directory `$CARGO_HOME` (default `~/.cargo`) exists.
- `locations`:
  1. `registry/index` (bare git repo of registry metadata), `registry/cache` (downloaded `.crate` archives), `registry/src` (unpacked sources rustc reads).
     - `discover`: no command; compute `${CARGO_HOME:-$HOME/.cargo}`. Env override: `CARGO_HOME`. Default macOS: `~/.cargo`; Linux: `~/.cargo`.
  2. `git/db` (bare clones of git dependencies), `git/checkouts` (working checkouts of commits from `git/db`). Same root, same override.
  - NOT cache, same root: `bin/` (installed executables incl. rustup proxies), `config.toml`, `credentials.toml`, `.crates.toml`, `.crates2.json`. A cleaner must target only `registry/` and `git/`, never the root. Non-Cargo content may also sit in the root, e.g. `advisory-db/` (cargo-audit), `env`.
- `scope`: global
- `measure`: `du -sk <CARGO_HOME>/registry <CARGO_HOME>/git`
- `clean`: delete directory (the subdirectories `registry/cache`, `registry/src`, `registry/index`, `git/checkouts`, `git/db`). Docs: "In theory, you can remove any part of the cache. Cargo will try to restore what a crate needs." `cargo clean` does NOT touch it ("Once a crate is cached it is not removed by the clean command"). No native full-wipe command. Prune-unused: Cargo does automatic GC: network-downloaded files unused for 3 months and regenerable-offline files unused for 1 month are deleted, checked at most once per `cache.auto-clean-frequency` (default `"1 day"`, env `CARGO_CACHE_AUTO_CLEAN_FREQUENCY`; values `never`, `always`, `N seconds|minutes|hours|days|weeks|months`); disabled with `--offline`/`--frozen`. A manual GC subcommand (`cargo clean gc`, behind `-Z gc`) is `unverified`; local `cargo --list` shows only plain `clean`. Third-party `cargo-cache` crate exists (docs mention it); its flags are `unverified`.
  - Notes: no read-only file problem known (unverified); deleting `registry/index` forces a full index re-fetch.
- `regenerable`: `yes-but-large-download` (index, crates, git repos re-fetched; git dependencies pinned to a rev that was force-pushed away or a deleted repo or yanked crates can become unrecoverable offline; offline use breaks).
- `in_use_hazard`: a running `cargo build` reading `registry/src` or `git/checkouts` can fail with missing-file errors; cargo re-extracts on the next run. Cargo uses a package-cache lock (unverified).
- `sources`: https://doc.rust-lang.org/cargo/guide/cargo-home.html ; https://doc.rust-lang.org/cargo/reference/environment-variables.html ; https://doc.rust-lang.org/cargo/reference/config.html (section `cache.auto-clean-frequency`)
- `verification`: V(discover: official-docs (env/default rule) / default-paths: official-docs; ran-command for `~/.cargo` existing / clean: official-docs for "can be removed", `unverified` for `cargo clean gc` and cargo-cache flags)

## cargo (build output / target)
- `name`: Cargo build output; `ecosystem`: Rust
- `detect`: `cargo` on PATH; per project a `Cargo.toml`.
- `locations`:
  1. The target directory (final artifacts: `debug/`, `release/`, `<profile>/`, `<triple>/debug|release`, `doc/`, `package/`).
  2. The build directory (intermediate: `<build-dir>/debug/deps`, `incremental`, `build`). Config key `build.build-dir`, env `CARGO_BUILD_BUILD_DIR`, defaults to the value of `build.target-dir`; supports templates `{workspace-root}`, `{cargo-cache-home}`, `{workspace-path-hash}`. (Newer Cargo; Cargo 1.93 emits a `build_directory` field in metadata; behaviour on older Cargo `unverified`.)
  - `discover` (the exact effective path, in precedence order, all `official-docs` unless noted):
    1. CLI `--target-dir <DIR>` (only when ccc itself invokes cargo).
    2. Env `CARGO_TARGET_DIR`, or `CARGO_BUILD_TARGET_DIR` (both documented; precedence between the two `unverified`). Relative values resolve against the current working directory.
    3. Config key `build.target-dir`, merged from files probed starting at the cwd and walking each parent directory: `<dir>/.cargo/config.toml` (also extensionless `.cargo/config`; if both exist the extensionless file is used), ending with `$CARGO_HOME/config.toml` (default `~/.cargo/config.toml`). Deeper directories win; the Cargo-home file is lowest priority. `--config KEY=VALUE` beats env beats files. A relative path in a config file resolves against the PARENT of the `.cargo` directory that defines it (so `target-dir = "build"` in `/p/.cargo/config.toml` means `/p/build`). When run from a workspace root Cargo does not read member-crate configs. Importantly, the search starts from the cwd of the cargo invocation, not from the manifest's directory.
    4. Default: directory `target` at the workspace root (the directory of the workspace-root `Cargo.toml`, not necessarily the member crate).
  - Single authoritative command (preferred over reimplementing 1-4): from the project directory run `cargo metadata --format-version 1 --no-deps` and read JSON field `target_directory` (and `build_directory`). When a shared target directory is configured, both fields point at it instead of `<project>/target`. Running it in a directory without a manifest fails (observed: empty output, parse error), so it needs a project path. A project-less tool (a user-level scan) can instead parse `$CARGO_HOME/config.toml` and env itself. `cargo config get build.target-dir` exists only behind `-Z unstable-options` (`unverified`).
  - Defaults: macOS and Linux both `<workspace-root>/target`.
- `scope`: decided at runtime per project, not per tool. If the effective target dir is `<workspace-root>/target` it is `per-project` (marker: sibling `Cargo.toml` in the parent, and a `CACHEDIR.TAG` file inside the target dir, which Cargo writes - the `CACHEDIR.TAG` claim is `unverified`). If `build.target-dir` / `CARGO_TARGET_DIR` points elsewhere (for example set in the user-level config) it is one `global` directory shared by many projects: it has no sibling `Cargo.toml`, so walking projects will never find it; it must be found via config/env/`cargo metadata` and deduplicated across projects. A project-level `.cargo/config.toml` can also point several projects at one directory or give each a custom relative path. Shared-dir marker candidates: `CACHEDIR.TAG`, `.rustc_info.json` (`unverified`).
- `measure`: `du -sk <target_directory>`. Per-profile: `du` of `<dir>/debug`, `<dir>/release`.
- `clean`: full wipe: `cargo clean` (official: "With no options, it deletes the entire target directory") or `cargo clean --target-dir <DIR>` / `cargo clean --manifest-path <Cargo.toml>`. Partial variants (all official): `-p <spec>`, `--workspace`, `--release`, `--profile <name>`, `--doc`, `--target <triple>`, `--dry-run` (list what would be deleted; combine with `-v` to list files; flag also confirmed by `cargo clean --help`). No prune-unused command in Cargo itself; third-party `cargo-sweep` (installed to `~/.cargo/bin/cargo-sweep`) exists, its flags are `unverified`.
  - Notes: with a shared target dir, one `cargo clean` from ANY project wipes the artifacts of ALL projects using it (the docs do not discuss shared dirs; the whole-directory behaviour follows from "deletes the entire target directory"). If ccc cleans by `du`-and-delete of the discovered path, delete it once, not once per project. `cargo clean -p` in a shared dir only removes that package's artifacts across all profiles.
- `regenerable`: `yes-but-slow-rebuild` (can be tens of GiB, especially in a shared dir).
- `in_use_hazard`: a running `cargo build`/`cargo test`/`cargo watch`/rust-analyzer check writes here; deleting mid-build fails the build; Cargo takes a lock on the build directory so `cargo clean` can block waiting (`unverified`). Shared dir raises the chance that some other project is mid-build.
- `sources`: https://doc.rust-lang.org/cargo/reference/config.html ; https://doc.rust-lang.org/cargo/reference/environment-variables.html ; https://doc.rust-lang.org/cargo/commands/cargo-clean.html ; https://doc.rust-lang.org/cargo/reference/build-cache.html
- `verification`: V(discover: official-docs + ran-command (`cargo metadata`) / default-paths: official-docs / clean: official-docs (`cargo clean`, flags), ran-command (`--dry-run` flag listed in `cargo clean --help`); `CACHEDIR.TAG` marker: unverified)

## rustup (toolchains and downloads)
- `name`: rustup; `ecosystem`: Rust
- `detect`: `rustup` on PATH (`~/.cargo/bin/rustup`); directory `$RUSTUP_HOME` exists.
- `locations`:
  1. `$RUSTUP_HOME/toolchains/<name>` installed toolchains. `discover`: `rustup show home` (prints e.g. `~/.rustup`); `rustup toolchain list -v` prints each toolchain with its path (e.g. `stable-aarch64-apple-darwin (active, default) ~/.rustup/toolchains/stable-aarch64-apple-darwin`). Override env `RUSTUP_HOME`. Default macOS and Linux: `~/.rustup`.
  2. `$RUSTUP_HOME/downloads`, `$RUSTUP_HOME/tmp`, `$RUSTUP_HOME/update-hashes` (their purpose being disposable download staging and hash records is `unverified`). `settings.toml` (default toolchain, directory overrides) lives in the same root and is configuration, not cache.
- `scope`: global
- `measure`: `du -sk` of each toolchain directory; `du` of `downloads` and `tmp`.
- `clean`: per-toolchain: `rustup toolchain uninstall <toolchain>` (subcommand `uninstall` under `toolchain` confirmed by `rustup --help`: "toolchain  Install, uninstall, or list toolchains"; exact argument form `unverified`). Full wipe of everything: `rustup self uninstall` (destroys rustup and all toolchains; NOT recommended for a cache cleaner). `downloads` and `tmp`: delete directory (`unverified`). No prune-unused command in rustup.
  - Notes: removing the default toolchain breaks the `cargo`/`rustc` proxies until another is set (`rustup default`); removing a toolchain referenced by a `rust-toolchain.toml` makes the next cargo call in that project re-download it.
- `regenerable`: `no` for the set of toolchains as a user choice (which channels, targets, components, pinned dates are decisions someone made); technically `yes-but-large-download`. `downloads`/`tmp`: `yes`.
- `in_use_hazard`: uninstalling a toolchain used by a running build or by rust-analyzer breaks it immediately (the compiler binary vanishes).
- `sources`: https://rust-lang.github.io/rustup/environment-variables.html ; https://rust-lang.github.io/rustup/concepts/toolchains.html ; `rustup --help`
- `verification`: V(discover: ran-command / default-paths: official-docs (`~/.rustup`) / clean: `unverified` for exact `uninstall` argument syntax, ran-command for subcommand existence)

## sccache
- `name`: sccache; `ecosystem`: Rust/C/C++ (compiler cache)
- `detect`: `sccache` on PATH ; directory exists.
- `locations`:
  1. Local disk cache. `discover`: `sccache --show-stats` prints a line `Cache location  Local disk: "<path>"` (e.g. `~/Library/Caches/Mozilla.sccache` on macOS) and `Max cache size  10 GiB`. Override env `SCCACHE_DIR`; size limit `SCCACHE_CACHE_SIZE` (default 10G). Default macOS `~/Library/Caches/Mozilla.sccache`; Linux `~/.cache/sccache`. If a remote backend (S3, GCS, Redis, ...) is configured, "Cache location" shows that instead and there is nothing local to measure or delete.
- `scope`: global
- `measure`: `du -sk <path>`. `sccache --show-stats` prints hit/miss counters; in the observed 0.14.0 output with zero compilations there was NO current-size line (only "Max cache size"), so use `du`. `sccache --show-adv-stats` also exists (exit 0).
- `clean`: no native clear-cache command (the README documents `--show-stats`, `--stop-server`; `--zero-stats` is in `sccache --help`, which resets counters only). Procedure: `sccache --stop-server`, then delete directory. Prune-unused: sccache self-evicts to `SCCACHE_CACHE_SIZE` (LRU behaviour `unverified`).
  - Notes: stop-server shuts the daemon (listens on 127.0.0.1:4226 by default, `SCCACHE_SERVER_PORT`; exits after 10 minutes idle by default).
- `regenerable`: `yes-but-slow-rebuild` (it is a compile cache).
- `in_use_hazard`: while `RUSTC_WRAPPER=sccache` builds run, the server owns the directory; deleting under it causes cache write errors or compile failures (exact failure mode `unverified`); stop the server first.
- `sources`: https://github.com/mozilla/sccache/blob/main/README.md ; https://github.com/mozilla/sccache/blob/main/docs/Local.md
- `verification`: V(discover: ran-command / default-paths: official-docs + ran-command (macOS) / clean: official-docs for `--stop-server`; delete-directory is the only option)

## cargo install binaries / cargo-binstall
- `name`: cargo-installed binaries; `ecosystem`: Rust
- `detect`: `$CARGO_HOME/bin` exists; `cargo-binstall` on PATH for binstall.
- `locations`:
  1. `$CARGO_HOME/bin` executables from `cargo install` AND rustup proxies (`cargo`, `rustc`, `rustup`, `rust-analyzer`, ... are symlinks to `rustup`). `discover`: `${CARGO_HOME:-$HOME/.cargo}/bin`. Metadata: `.crates.toml`, `.crates2.json` in the same root.
  2. cargo-binstall's own download/cache directory: location and clean command `unverified` (not researched).
- `scope`: global
- `measure`: `du -sk` of `bin/`.
- `clean`: per binary `cargo uninstall <crate>` (`unverified`; not in fetched docs); list with `cargo install --list` (`unverified`). Not a cache: do not wipe `bin/`.
- `regenerable`: `no — user-installed tools` (the set of tools someone chose; re-install needs compile or download). Removing rustup proxies breaks the toolchain.
- `in_use_hazard`: removing a binary that is running is allowed on Unix but breaks next invocation.
- `sources`: https://doc.rust-lang.org/cargo/guide/cargo-home.html
- `verification`: V(discover: official-docs / default-paths: official-docs / clean: unverified)
- Recommendation: classify as `report-only / never auto-clean`.

## rust-analyzer
- `name`: rust-analyzer; `ecosystem`: Rust (language server)
- `detect`: `rust-analyzer` on PATH (rustup proxy at `~/.cargo/bin/rust-analyzer`).
- `locations`: no persistent cache of its own was found in the researched material. It runs `cargo check`/build scripts into a Cargo target dir (by default the project's target dir; a separate `rust-analyzer.cargo.targetDir` setting exists - `unverified`), so its disk usage is part of the Cargo target entry above. Editor-side caches (VS Code workspace storage) are out of scope.
- `scope`: n/a (covered by cargo target)
- `measure`/`clean`: see cargo target. `regenerable`: `yes-but-slow-rebuild`. `in_use_hazard`: wiping target while the server runs forces a full re-check and can show transient errors.
- `sources`: none fetched. `verification`: all `unverified`.
- Recommendation: do not create a separate registry entry unless an own-cache is confirmed.

---

## go (build cache, test cache, fuzz cache)
- `name`: go build cache; `ecosystem`: Go
- `detect`: `go` on PATH.
- `locations`:
  1. Build cache (`GOCACHE`): compiled packages and cached test results. `discover`: `go env GOCACHE` (prints e.g. `~/Library/Caches/go-build` on macOS). Must be an absolute path. Override: env `GOCACHE` (or `go env -w GOCACHE=...`, written to the Go env file). Default: "a subdirectory named go-build in the standard user cache directory for the current operating system": macOS `~/Library/Caches/go-build`, Linux `$XDG_CACHE_HOME/go-build` else `~/.cache/go-build` (derived from the documented rule; the literal Linux path is `unverified`). `GOCACHEPROG` can redirect caching to an external program, in which case the directory may be unused.
  2. Test cache: stored inside the build cache (not a separate directory). Cleaned by `go clean -testcache`.
  3. Fuzz cache: `go clean -fuzzcache` removes "fuzzing data from the build cache"; physical location `$GOCACHE/fuzz` is `unverified`. Project `testdata/fuzz` seeds are NOT affected (official).
- `scope`: global
- `measure`: `du -sk "$(go env GOCACHE)"`.
- `clean`: full wipe `go clean -cache`; tests only `go clean -testcache` (expires results, keeps rest); fuzz only `go clean -fuzzcache`. The `go clean` usage line (`go clean [-i] [-r] [-cache] [-testcache] [-modcache] [-fuzzcache]`) confirmed by `go help clean`. Prune-unused: the go command itself "periodically deletes cached data that hasn't been used recently"; a specific retention period (commonly cited as 5 days) is `unverified`.
- `regenerable`: `yes-but-slow-rebuild`. Fuzz cache: `yes` but "may make fuzzing less effective until new inputs are found" (official) - corpus found by long fuzz runs is lost.
- `in_use_hazard`: `go build`/`go test`/gopls running during `go clean -cache` can see cache misses or transient errors; generally recoverable (`unverified`).
- `sources`: https://pkg.go.dev/cmd/go (sections `go clean`, "Build and test caching", environment variables)
- `verification`: V(discover: ran-command / default-paths: ran-command (macOS), official-docs (rule), unverified (literal Linux) / clean: official-docs + ran-command (`go help clean` usage))

## go (module cache)
- `name`: go module cache; `ecosystem`: Go
- `detect`: `go` on PATH.
- `locations`:
  1. `GOMODCACHE`: downloaded module sources (`<modcache>/<module>@<version>`) and, under `cache/download`, the module proxy download cache (the `cache/download` subpath is `unverified`). `discover`: `go env GOMODCACHE` (prints e.g. `~/go/pkg/mod`). Override: env `GOMODCACHE`. Default: `$GOPATH/pkg/mod` (official); `GOPATH` default `$HOME/go` (official, implied via GOBIN default). macOS and Linux both `~/go/pkg/mod`. `GOPATH` can be a list; the module cache uses its first element (`unverified`). `$GOPATH/bin` (installed commands) is NOT cache and sits next to it.
- `scope`: global
- `measure`: `du -sk "$(go env GOMODCACHE)"`.
- `clean`: `go clean -modcache` ("removes the entire module download cache, including unpacked source code of versioned dependencies"). Note (official): by default the module cache is created read-only (directories lack write permission); plain `rm -r` fails without `chmod -R u+w` first. Setting `GOFLAGS=-modcacherw` (e.g. `go env -w GOFLAGS=-modcacherw`) makes new entries writable. `go clean -modcache` "may be used to delete the module cache, whether or not -modcacherw was used", so ccc must prefer the native command, or chmod before deleting. No prune-unused command.
- `regenerable`: `yes-but-large-download` (needs network/proxy; private modules require credentials; modules removed from proxy can be gone).
- `in_use_hazard`: running builds/`go list`/gopls lose sources; a build fails with missing module until re-download; offline machines cannot recover (`GOFLAGS=-mod=mod`/`GOPROXY=off` setups especially).
- `sources`: https://go.dev/ref/mod (module cache, `-modcacherw`) ; https://pkg.go.dev/cmd/go
- `verification`: V(discover: ran-command / default-paths: official-docs + ran-command / clean: official-docs)

## gopls
- `name`: gopls; `ecosystem`: Go (language server)
- `detect`: `gopls` on PATH or `$GOPATH/bin/gopls`.
- `locations`: on-disk cache under the user cache dir, `gopls` subdirectory (macOS `~/Library/Caches/gopls`, Linux `~/.cache/gopls`) - `unverified` (not fetched). `discover`: no known command (`gopls` has no documented path command; `unverified`). Env override: none known (`unverified`).
- `scope`: global
- `measure`: `du` of the path. `clean`: delete directory; no native command known (`unverified`). `regenerable`: `yes` (re-indexed). `in_use_hazard`: a running editor session holds the index open; stop/restart the language server.
- `sources`: none fetched. `verification`: all `unverified`.

## golangci-lint
- `name`: golangci-lint; `ecosystem`: Go
- `detect`: `golangci-lint` on PATH.
- `locations`:
  1. Cache directory. Default: a `golangci-lint` subdirectory of Go's `os.UserCacheDir()` (macOS `~/Library/Caches/golangci-lint`, Linux `~/.cache/golangci-lint`, derived from `os.UserCacheDir` semantics, literal paths `unverified`). Override env `GOLANGCI_LINT_CACHE` (must be an absolute path). `discover`: `golangci-lint cache status` (documented as "shows the cache status"; whether it prints the path/size is `unverified`).
- `scope`: global. `measure`: `du` of the path.
- `clean`: `golangci-lint cache clean` (official). No prune variant documented.
- `regenerable`: `yes-but-slow-rebuild`. `in_use_hazard`: concurrent lint run loses cache entries (cache only used by `golangci-lint run`, official); low risk.
- `sources`: https://golangci-lint.run/docs/configuration/cli/
- `verification`: V(discover: unverified / default-paths: official-docs for rule, unverified for literal / clean: official-docs)

---

## ccache
- `name`: ccache; `ecosystem`: C/C++
- `detect`: `ccache` on PATH; or directory exists.
- `locations`:
  1. `cache_dir`. `discover`: `ccache -k cache_dir` (`--get-config cache_dir`, official); `ccache -p` / `--show-config` lists all values and their source; `ccache -s` shows it too. Override: env `CCACHE_DIR` or `-d PATH`; config file location env `CCACHE_CONFIGPATH`. Default (non-Windows, first match): `$HOME/.ccache` if that directory exists (legacy); else `$XDG_CACHE_HOME/ccache` if `XDG_CACHE_HOME` set; else `$HOME/Library/Caches/ccache` on macOS, `$HOME/.cache/ccache` elsewhere. `max_size` default 5 GiB (`CCACHE_MAXSIZE`). A `ccache.conf` may live in the cache dir; `-C` keeps config.
- `scope`: global
- `measure`: `ccache -s` (`--show-stats`; `-v` for more detail) reports cache size and hit rate; or `du -sk <cache_dir>`.
- `clean`: full wipe `ccache -C` (`--clear`: removes all cached files, keeps configuration). Prune-unused: `ccache -c` (`--cleanup`: removes less recently used files until limits are met, recalculates totals). Reset counters only: `ccache -z`.
- `regenerable`: `yes-but-slow-rebuild`.
- `in_use_hazard`: concurrent compiles through ccache while clearing - concurrency safety is `unverified` (not in fetched manual section); expect only cache misses.
- `sources`: https://ccache.dev/manual/latest.html
- `verification`: V(discover: official-docs / default-paths: official-docs / clean: official-docs)

## conan (v2)
- `name`: Conan 2; `ecosystem`: C/C++ package manager
- `detect`: `conan` on PATH; `~/.conan2` exists.
- `locations`:
  1. Conan home: `CONAN_HOME` env; default `<user home>/.conan2` (official). Contains profiles, remotes config, settings, and the package cache (`p/` subfolder, `unverified`). `discover`: `conan config home` (`unverified`; the fetched page did not cover it). Default macOS/Linux: `~/.conan2`.
  2. Download cache (optional): configured by `core.download:download_cache` (`unverified`); no default.
- `scope`: global
- `measure`: `du -sk <conan home>/p` (`unverified` path); per-package: `conan cache path <reference> [--folder {export_source,source,build,metadata}]` (official syntax; takes a reference).
- `clean`: `conan cache clean [pattern]` with options `-s/--source`, `-b/--build`, `-d/--download`, `-t/--temp`, `-bs/--backup-sources`, `-l/--list LIST`, `-p/--package-query` (all official). This removes temporary folders (build/source/download/temp), not complete packages. Removing packages: `conan remove <pattern>` (page fetched only showed `conan remove --list pkglist.json`; full syntax incl. confirm flag `-c` is `unverified`). `conan cache check-integrity` exists (official).
  - Notes: the home also holds user configuration (profiles, `remotes.json`, `global.conf`): do not delete the whole home.
- `regenerable`: build/source/download/temp folders `yes-but-slow-rebuild` or `yes`; packages in the cache `yes-but-large-download` EXCEPT packages created locally (`conan create`/`export`) and never uploaded to a remote: `no — contains user data` (locally built recipes and binaries not available elsewhere).
- `in_use_hazard`: the package cache is not safe for concurrent processes (official: "not concurrent"; use separate `CONAN_HOME` per parallel job), so cleaning during a `conan install/create` can corrupt it.
- `sources`: https://docs.conan.io/2/reference/commands/cache.html ; https://docs.conan.io/2/reference/environment.html
- `verification`: V(discover: unverified / default-paths: official-docs (`~/.conan2`; `<home>/p` unverified) / clean: official-docs for `conan cache clean`, unverified for `conan remove` flags)

## conan (v1)
- `name`: Conan 1.x; `ecosystem`: C/C++
- `detect`: `conan --version` major 1; `~/.conan` exists.
- `locations`: home `~/.conan` (override `CONAN_USER_HOME`, which sets the parent of `.conan`; `unverified`), package storage `~/.conan/data` (`unverified`), configurable via `storage.path` in `conan.conf` / `CONAN_STORAGE_PATH` (`unverified`). Short-path build dirs (`short_paths`, Windows) not relevant. `discover`: `conan config home` (`unverified`), `conan config get storage.path` (`unverified`).
- `scope`: global. `measure`: `du`.
- `clean`: `conan remove "*" -f` (all packages), `conan remove "*" --builds --src -f` (build and source folders only) - `unverified` (docs page returned 404).
- `regenerable`: same as v2: `yes-but-large-download`, locally created packages are `no — contains user data`.
- `in_use_hazard`: as v2.
- `sources`: https://docs.conan.io/1/ (page for config returned 404; not read). `verification`: all `unverified`.
- Contract note: v1 and v2 are both binary `conan`; the registry must branch on `conan --version`.

## vcpkg
- `name`: vcpkg; `ecosystem`: C/C++
- `detect`: `vcpkg` on PATH or `VCPKG_ROOT` env set / directory with `vcpkg` bootstrap.
- `locations`:
  1. Default binary cache (archives of built packages). Default (official, non-Windows, first valid): `$VCPKG_DEFAULT_BINARY_CACHE`, then `$XDG_CACHE_HOME/vcpkg/archives`, then `$HOME/.cache/vcpkg/archives`. This applies to macOS too (it is listed as "Non-Windows"; macOS does NOT use `~/Library/Caches`). No `discover` command known (`vcpkg help binarycaching` prints in-tool help; whether it prints the resolved path is `unverified`). Caches configured via `VCPKG_BINARY_SOURCES` can point elsewhere (NuGet, cloud) (`unverified`).
  2. In the vcpkg root (a git clone, must not be deleted whole): `downloads/` (source tarballs and tools), `buildtrees/` (build working dirs and logs), `packages/` (staging) - names `unverified`; `installed/` (installed packages used by projects in classic mode) is not cache.
  3. Manifest mode: per project `vcpkg_installed/` next to `vcpkg.json` (`unverified`).
- `scope`: binary cache global; root dirs global (location of root from `VCPKG_ROOT`); `vcpkg_installed` per-project (marker: sibling `vcpkg.json`, `unverified`).
- `measure`: `du -sk`.
- `clean`: no native command to clear the binary cache found; delete directory. `vcpkg remove <port>` removes installed ports (`unverified`). `buildtrees`/`downloads`: delete contents (`unverified`).
- `regenerable`: binary cache `yes-but-slow-rebuild`; downloads `yes-but-large-download`; `installed/` `yes-but-slow-rebuild`.
- `in_use_hazard`: deleting `installed/` or the binary cache during a CMake configure/build using the vcpkg toolchain file breaks it.
- `sources`: https://learn.microsoft.com/en-us/vcpkg/users/binarycaching
- `verification`: V(discover: unverified / default-paths: official-docs for binary cache, unverified for root subdirs / clean: unverified)

## bazel (and bazelisk)
- `name`: Bazel / Bazelisk; `ecosystem`: C/C++ and polyglot build
- `detect`: `bazel` or `bazelisk` on PATH; `WORKSPACE`/`MODULE.bazel` in a project (marker `unverified`).
- `locations`:
  1. Output base (per workspace; build outputs, execroot, external repositories). `discover`: `bazel info output_base` (official definition: "absolute path to the base output directory that this Bazel instance uses for the current user and workspace"); must be run inside the workspace. Override: startup option `--output_base`. Default per docs generated text: `${OUTPUT_ROOT}/blaze${USER}/${MD5_OF_WORKSPACE_ROOT}` (the reference's generic form); concrete OS paths (Linux `~/.cache/bazel/_bazel_$USER/<md5>`, macOS `/private/var/tmp/_bazel_$USER/<md5>`) are `unverified`. `--output_user_root` sets the parent ("user-specific directory under which all build outputs are written", official).
  2. Repository cache (downloaded external archives). Default `{--output_user_root}/cache/repos/v1` (official); override `--repository_cache` (empty string disables). `bazel info repository_cache` is `unverified`.
  3. Disk cache (opt-in action cache). `--disk_cache=<path>`; a bare `--disk_cache` uses `<outputUserRoot>/cache/disk` (official). Bazel 7.4+ can garbage-collect it via `--experimental_disk_cache_gc_max_size` / `--experimental_disk_cache_gc_max_age` while idle (default idle delay 5 minutes, official). Configured usually in `.bazelrc` (`~/.bazelrc`, workspace `.bazelrc`), so the path can only be learned by reading rc files (`unverified`).
  4. Bazelisk binary downloads: macOS `~/Library/Caches/bazelisk`, Linux `~/.cache/bazelisk`, override `BAZELISK_HOME` (`unverified`).
  - Workspace root contains `bazel-bin`, `bazel-out`, `bazel-<workspace>` convenience symlinks into the output base (`unverified`).
- `scope`: output base is `global` in storage (under the user root) but keyed per workspace via md5 of path - it cannot be found by walking project directories and a deleted project leaves an orphan; must be asked via `bazel info` from each workspace, or enumerated under the output user root. Repository cache: `global`. Disk cache: `global` (if configured).
- `measure`: `du -sk "$(bazel info output_base)"`; `du` of repository/disk cache.
- `clean`: `bazel clean` (official: deletes output directories for all build configurations; keeps the output base and the repository cache); `bazel clean --expunge` (official: removes the entire output base tree, including build output and all temp files, and stops the server); `bazel clean --expunge_async` (official: runs the expunge in the background). Repository and disk caches: delete directory (Bazel has no native wipe for them; GC flags above for disk cache). `bazel shutdown` stops the server (`unverified` command text; `--expunge` stops it implicitly).
- `regenerable`: `yes-but-slow-rebuild` (output base), `yes-but-large-download` (repository cache; external archives that are no longer hosted cannot be refetched).
- `in_use_hazard`: a Bazel server (daemon) is bound to each output base and holds a lock ("each Bazel server process can handle at most one invocation at a time"); deleting the output base under a live server breaks it. Stop with `bazel shutdown` or use `clean --expunge` which stops it.
- `sources`: https://bazel.build/docs/user-manual ; https://bazel.build/reference/command-line-reference ; https://bazel.build/remote/caching ; https://bazel.build/run/scripts
- `verification`: V(discover: official-docs (`info output_base`) / default-paths: official-docs for repository and disk cache, unverified for OS-specific output root and bazelisk / clean: official-docs)

## buck2
- `name`: Buck2; `ecosystem`: C/C++ and polyglot
- `detect`: `buck2` on PATH; project marker `.buckconfig` at the repo root (official: Buck2 reads `.buckconfig`).
- `locations`:
  1. `buck-out` directory in the project root (official). Layout is unstable; do not assume paths inside.
  2. Daemon state under `.buckd` (official mentions it as the daemon's directory); precise location (`buck-out/<isolation>/...` vs home) `unverified`.
- `scope`: `per-project` (marker: `.buckconfig` sibling of `buck-out`). `discover`: path is fixed relative to the project root; `buck2 root` prints the root (`unverified`).
- `measure`: `du -sk buck-out`.
- `clean`: `buck2 clean` (official: deletes generated files and caches; also kills the buck2 daemon). Options (official): `--dry-run`, `--background`, `--stale <DURATION>` (deletes stale artifacts without killing the daemon: the prune-unused variant), `--tracked-only`, `--adaptive-low-disk-threshold <PERCENT>`, `--adaptive-min-ttl <DURATION>` (default 12h), `--adaptive-unmaterialize-active`.
- `regenerable`: `yes-but-slow-rebuild` (remote cache may repopulate it).
- `in_use_hazard`: the daemon holds `buck-out`; `buck2 clean` handles that by killing it (except `--stale`). Raw `rm -rf buck-out` under a running daemon is unsafe. Whether it is safe to delete `buck-out` at all is not stated on the concepts page.
- `sources`: https://buck2.build/docs/users/commands/clean/ ; https://buck2.build/docs/concepts/buck_out/ ; https://buck2.build/docs/concepts/buckconfig/
- `verification`: V(discover: official-docs (path fixed) / default-paths: official-docs / clean: official-docs)

## cmake FetchContent / CPM
- `name`: CMake FetchContent and CPM.cmake; `ecosystem`: C/C++
- `detect`: `cmake` on PATH; per-project `CMakeCache.txt`; CPM: env `CPM_SOURCE_CACHE`.
- `locations`:
  1. FetchContent default: `<build-dir>/_deps` (`FETCHCONTENT_BASE_DIR` defaults to `${CMAKE_BINARY_DIR}/_deps`) - `unverified` (not fetched). Per build directory, marker: `CMakeCache.txt` beside `_deps/` (`unverified`).
  2. CPM source cache: `CPM_SOURCE_CACHE` (env var, or `-DCPM_SOURCE_CACHE=<path>` which overrides env; official). NO default: official page states none, only the example `export CPM_SOURCE_CACHE=$HOME/.cache/CPM`. Directory per package named by a hash of the `CPMAddPackage()` arguments. `discover`: env var only; or the cached variable in a project's `CMakeCache.txt` (`unverified`).
- `scope`: FetchContent `per-project` (build dirs; build dirs can be anywhere, e.g. `build/`, `cmake-build-*`, found by `CMakeCache.txt`); CPM `global`, but exists only if the user set the variable.
- `measure`: `du`.
- `clean`: no native command (CPM docs give no cleanup instructions - official). Delete directory. FetchContent: delete `_deps` or the build directory.
- `regenerable`: `yes-but-large-download`. `in_use_hazard`: a configure/build in progress reads sources from there; deleting breaks the build; patched sources in `_deps` lost.
- `sources`: https://github.com/cpm-cmake/CPM.cmake ; FetchContent doc not fetched.
- `verification`: V(discover: official-docs for CPM env / default-paths: official-docs (CPM none), unverified (FetchContent) / clean: official-docs (no command))

---

## zig
- `name`: Zig; `ecosystem`: Zig
- `detect`: `zig` on PATH; per-project `build.zig`.
- `locations`:
  1. Local cache `.zig-cache` in the project root (official; older releases used `zig-cache`, `unverified`). Official: it "can be completely deleted at any time with no consequences" (it only speeds up builds). Override: `--cache-dir [path]` (official: "Override path to local Zig cache directory"); env `ZIG_LOCAL_CACHE_DIR` (`unverified`).
  2. Global cache (compiled artifacts of std/libc, and the dependency package cache `p/`): `discover`: `zig env` prints `global_cache_dir` (`unverified`). Override: `--global-cache-dir` flag (appears in logged commands only, e.g. `/home/ci/.cache/zig`) or env `ZIG_GLOBAL_CACHE_DIR` (`unverified`). Defaults: Linux `~/.cache/zig`, macOS `~/Library/Caches/zig` (`unverified`).
- `scope`: local `per-project` (marker: sibling `build.zig`, `unverified`); global `global`.
- `measure`: `du`. `clean`: delete directory (no native command; `zig build` has none that is documented here). `regenerable`: local `yes-but-slow-rebuild`; global cache `yes-but-large-download` (package `p/` fetched again; hashes verify).
- `in_use_hazard`: concurrent `zig build` uses file-lock-protected cache entries; deleting mid-build can fail the build (`unverified`).
- `sources`: https://ziglang.org/learn/build-system/ ; https://ziglang.org/documentation/master/ (cache section not present in fetched text)
- `verification`: V(discover: unverified / default-paths: official-docs for `.zig-cache`, unverified for global / clean: official-docs (safe to delete local))

## nim / nimble
- `name`: Nim and Nimble; `ecosystem`: Nim
- `detect`: `nim` on PATH; `~/.nimble` exists.
- `locations`:
  1. nimcache: official: Posix `$XDG_CACHE_HOME/nim/$projectname(_r|_d)` else `~/.cache/nim/$projectname(_r|_d)` (`_r` release, `_d` debug). On macOS this is `~/.cache/nim/...` too, not `Library/Caches`. Windows `$HOME\nimcache\...` (out of scope). Override `--nimcache:PATH`. One directory per project name (names collide for same-named projects), all under one global parent.
  2. Nimble: `~/.nimble` (packages in `pkgs`/`pkgs2`, executables in `bin`) and env `NIMBLE_DIR` - `unverified`. `bin` holds user-installed tools; packages are user-chosen dependencies.
- `scope`: nimcache `global` parent with per-project children (not discoverable by walking projects; enumerate children). Nimble `global`.
- `measure`: `du`. `clean`: nimcache: delete directory (official: keeping generated files in one place "makes it easy to delete all generated files"). Nimble: `nimble uninstall <pkg>` (`unverified`).
- `regenerable`: nimcache `yes-but-slow-rebuild` (C compile); nimble packages `yes-but-large-download`, `bin/` `no — user installed tools`.
- `in_use_hazard`: running compile of that project.
- `sources`: https://nim-lang.org/docs/nimc.html
- `verification`: V(discover: unverified / default-paths: official-docs (nimcache), unverified (nimble) / clean: official-docs (delete))

## haskell stack
- `name`: Stack; `ecosystem`: Haskell
- `detect`: `stack` on PATH; `~/.stack`.
- `locations`:
  1. Stack root. `discover`: `stack path --stack-root` (official). Override: env `STACK_ROOT`, flag `--stack-root`; `STACK_XDG` non-empty switches to `<XDG_DATA_HOME>/stack` (default `~/.local/share/stack`). Default: Unix `~/.stack` (official). Holds snapshots, installed GHCs under `programs/` (`unverified`), Pantry package cache (`unverified`), `config.yaml` (user config), `global-project`.
  2. `.stack-work` per project (build output). Official page covers `.stack-work` only for the global project in the root; the per-project meaning and marker (sibling `stack.yaml`) is `unverified`. `stack path --dist-dir` exists (official mention).
- `scope`: root `global`; `.stack-work` `per-project`.
- `measure`: `du`. `clean`: per project `stack clean [PACKAGE]` (project `dist` dirs), `stack clean --full` (deletes the Stack work directories of the project and its packages), `--omit-this` (official). For the root: delete `programs/` or snapshots by hand (no native command found; `unverified`).
- `regenerable`: project `yes-but-slow-rebuild`; root `yes-but-large-download` (GHC downloads); `config.yaml` is user config (do not delete).
- `in_use_hazard`: running build; deleting a GHC under `programs/` that a project resolves breaks until re-install.
- `sources`: https://docs.haskellstack.org/en/stable/topics/stack_root/ ; https://docs.haskellstack.org/en/stable/commands/clean_command/
- `verification`: V(discover: official-docs / default-paths: official-docs (root), unverified (subdirs) / clean: official-docs (`stack clean`))

## haskell cabal
- `name`: Cabal; `ecosystem`: Haskell
- `detect`: `cabal` on PATH; `~/.cabal` or XDG dirs.
- `locations`:
  1. `cabal path` prints directories for cache, remote repository cache, logs, store, config file and installed binaries (official); `cabal path --output-format=json`; one directory via a single path option such as `--installdir`. Example in docs shows `store-dir: /home/alice/.cabal/store`. Default roots: legacy `~/.cabal` (official example) and XDG layout `~/.config/cabal`, `~/.cache/cabal`, `~/.local/state/cabal` (`unverified`); env `CABAL_DIR` (`unverified`). Script builds under `$XDG_CACHE_HOME/cabal/script-builds/` (official).
  2. `dist-newstyle` per project (official).
- `scope`: global dirs + `per-project` `dist-newstyle` (marker: sibling `cabal.project` or `*.cabal`, `unverified`).
- `measure`: `du` of paths from `cabal path`. `clean`: `cabal clean` (removes `dist-newstyle`; with `--save-config` keeps configuration and caches and removes only build artifacts, official); `cabal clean path/to/script` for script artifacts (official). Global store: delete directory (no native gc known, `unverified`). Do not delete `~/.cabal/bin` (installed executables) or the `config` file.
- `regenerable`: `yes-but-slow-rebuild` (store, dist); remote repo cache `yes` (index re-download).
- `in_use_hazard`: running build; deleting store entries of a GHC version used by installed executables does not break those executables (statically linked, `unverified`).
- `sources`: https://cabal.readthedocs.io/en/stable/cabal-commands.html
- `verification`: V(discover: official-docs (`cabal path`) / default-paths: official-docs (partial) / clean: official-docs (`cabal clean`))

## ghcup
- `name`: GHCup; `ecosystem`: Haskell toolchain manager
- `detect`: `ghcup` on PATH; `~/.ghcup`.
- `locations`: root `~/.ghcup` (implied by official docs; override `GHCUP_INSTALL_BASE_PREFIX` is `unverified`). `~/.ghcup/cache` download cache (official: "All cached files end up in ~/.ghcup/cache by default"; tarballs cached only with `ghcup --cache` or `ghcup config set cache true`; metadata cached 5 minutes). Installed toolchains (GHC, cabal, stack, HLS) in subdirectories (`unverified`).
- `scope`: global. `measure`: `du`. `clean`: `ghcup gc --cache` (official; the other gc flags `--ghc-old`, `--profiling-libs`, `--share-dir`, `--hls-no-ghc`, `--tmpdirs` are `unverified`). Toolchains: `ghcup rm ghc <version>` (`unverified`), list with `ghcup list` (official).
- `regenerable`: cache `yes`; toolchains `no` as a user choice (`yes-but-large-download` technically).
- `in_use_hazard`: removing the set GHC breaks cabal/stack builds that use it.
- `sources`: https://www.haskell.org/ghcup/guide/
- `verification`: V(discover: unverified / default-paths: official-docs (cache), unverified (root override) / clean: official-docs (`gc --cache`))

## opam
- `name`: opam; `ecosystem`: OCaml
- `detect`: `opam` on PATH; `~/.opam`.
- `locations`:
  1. opam root: default `~/.opam` (official), override env `OPAMROOT` or `--root`. `discover`: global variable `root` (official: "the current opam root"); the `opam var root` command form is `unverified`.
  2. `$OPAMROOT/download-cache` (downloaded files), `$OPAMROOT/archives` (obsolete), `$OPAMROOT/repo` (repository mirrors), logs dir (all official). Switches `$OPAMROOT/<switch>` hold installed compilers/packages - user environments, not cache. Local switches: `_opam/` inside a project directory (official), marker sibling `*.opam` / `dune-project` (`unverified`).
- `scope`: global root; local switches `per-project` (`_opam`).
- `measure`: `du`; `opam clean --dry-run` shows removal commands without executing (official).
- `clean`: `opam clean` (default = `--logs --download-cache --switch-cleanup`). Flags (official): `-a/--all-switches`, `-c/--download-cache`, `--dry-run`, `--logs`, `-r/--repo-cache` (rebuilt when needed), `-s/--switch-cleanup` (removes backups, build dirs, uncompressed sources of non-dev packages, local metadata of previously pinned packages), `--untracked` (removes untracked files and directories: risky), `--unused-repositories`. Never run `--untracked` automatically.
- `regenerable`: download cache `yes-but-large-download`; repo cache `yes`; switches `no — user environments`.
- `in_use_hazard`: running opam commands lock the root; an `opam install` in progress uses build dirs that `--switch-cleanup` removes.
- `sources`: https://opam.ocaml.org/doc/man/opam-clean.html ; https://opam.ocaml.org/doc/Manual.html
- `verification`: V(discover: official-docs (partial) / default-paths: official-docs / clean: official-docs)

## elixir hex / mix
- `name`: Mix and Hex; `ecosystem`: Elixir/Erlang
- `detect`: `mix` on PATH; `~/.mix`, `~/.hex`.
- `locations`:
  1. `MIX_HOME` default `~/.mix` (official); `MIX_ARCHIVES` default `~/.mix/archives` (installed archives such as Hex, Phoenix installer: user-installed); `MIX_XDG` (opt-in XDG layout; precedence: `MIX_HOME` wins; exact XDG paths not stated, official). 
  2. Hex package cache and Hex home `~/.hex` (`HEX_HOME`, `~/.hex/packages`): `unverified` (the Hex page fetched did not mention them).
  3. Per project `_build` and `deps` (env `MIX_BUILD_PATH` / `MIX_DEPS_PATH` exist, `MIX_BUILD_ROOT` recommended for CI, official; default names `_build`/`deps` and marker sibling `mix.exs` `unverified`).
- `scope`: homes global; `_build`/`deps` `per-project`. The location can change with env, so discovery needs env inspection; `mix` has no documented path-printing command here (`unverified`).
- `measure`: `du`. `clean`: `mix clean` and `mix deps.clean --all` (`unverified`); else delete `_build` / `deps`. Do not delete `~/.mix/archives` blindly.
- `regenerable`: `_build` `yes-but-slow-rebuild`; `deps` `yes-but-large-download` (`mix deps.get`; path/git deps pinned in `mix.lock`); archives `no — user installed`.
- `in_use_hazard`: running `mix`/IEx/phoenix server (code reloader) breaks when `_build` disappears.
- `sources`: https://mix.hexdocs.pm/Mix.html ; https://hex.hexdocs.pm/Mix.Tasks.Hex.html
- `verification`: V(discover: unverified / default-paths: official-docs (`~/.mix`), unverified (Hex) / clean: unverified)

## rebar3 (Erlang)
- `name`: rebar3; `ecosystem`: Erlang/Elixir
- `detect`: `rebar3` on PATH; `~/.cache/rebar3`.
- `locations`:
  1. `~/.cache/rebar3` cache (official), override env `REBAR_CACHE_DIR`; Hex package cache subpath inside is `unverified`.
  2. `~/.config/rebar3` global configuration (official), override `REBAR_GLOBAL_CONFIG_DIR`; holds user config (`rebar.config`, Hex credentials - `unverified`) - NOT a cache. Both can also be moved with `{global_rebar_dir, "path"}`.
  3. `_build` per project (`base_dir` option, default `_build`; env `REBAR_BASE_DIR`, official; marker sibling `rebar.config`, `unverified`).
- `scope`: global + per-project. `measure`: `du`. `clean`: `rebar3 clean` flags (`--all`, `--profile`) `unverified`; else delete `_build`, delete `~/.cache/rebar3`.
- `regenerable`: `yes-but-large-download` (deps) / `yes-but-slow-rebuild`. `in_use_hazard`: running compile/shell.
- `sources`: https://rebar3.org/docs/configuration/configuration/
- `verification`: V(discover: unverified / default-paths: official-docs / clean: unverified)

## crystal shards
- `name`: Shards; `ecosystem`: Crystal
- `detect`: `shards` on PATH; per-project `shard.yml` (`unverified`).
- `locations`: `lib/` per project (official: `shards install` installs into the `lib` folder). Global cache `~/.cache/shards` and env `SHARDS_CACHE_PATH`: `unverified` (the man page returned no information on it).
- `scope`: `per-project` (marker: `shard.yml` + `shard.lock`, `unverified`) and global cache.
- `measure`: `du`. `clean`: `shards prune` (official: removes unused dependencies from the `lib` folder, i.e. the prune variant); full wipe: delete `lib/` and the global cache directory (no native command).
- `regenerable`: `yes-but-large-download` (`shards install`). `in_use_hazard`: running `crystal build`/spec compile reads `lib/`.
- `sources`: https://crystal-lang.org/reference/latest/man/shards/index.html
- `verification`: V(discover: unverified / default-paths: unverified / clean: official-docs (`shards prune`))

## dub (D)
- `name`: dub; `ecosystem`: D
- `detect`: `dub` on PATH; per-project `dub.json` / `dub.sdl` (`unverified`).
- `locations`: fetched packages "user wide shared location" (official; exact `~/.dub/packages` `unverified`; `--cache=local|system|user` selects the location class, official). Per-project build cache `.dub/` (`unverified`).
- `scope`: global + `per-project`. `measure`: `du`.
- `clean`: `dub clean [package]` (official: removes cached build files of the package, keeps the final target and copyFiles); `dub clean-caches` (official: removes cached metadata, e.g. package list and latest versions); `dub remove <package>` (official: removes a package cached on the local system); `dub list` lists local packages (official).
- `regenerable`: `yes-but-slow-rebuild` / `yes-but-large-download`. `in_use_hazard`: running `dub build`/`dub run`.
- `sources`: https://dub.pm/cli-reference/dub/
- `verification`: V(discover: unverified / default-paths: unverified / clean: official-docs)

---

## Contract observations

Patterns the registry contract must express that a "path + rm -rf" entry cannot:

1. **Path discoverable only by running a tool, with a documented override chain.** `go env GOCACHE`, `go env GOMODCACHE`, `rustup show home`, `sccache --show-stats` ("Cache location" line), `ccache -k cache_dir`, `stack path --stack-root`, `cabal path --output-format=json`, `bazel info output_base`. Each entry needs `discover` as an argv command plus a parse rule (plain stdout, a labelled line, a JSON field), the env/config override, and per-OS defaults only as a fallback (state when the fallback is allowed: only if the tool is absent, never when the tool is present but failed - fail closed).
2. **Scope decided at runtime, not by the tool.** Cargo `target` is per-project by default but one global directory when `build.target-dir`/`CARGO_TARGET_DIR` is set (all projects then share it). The effective path comes from a per-project command (`cargo metadata --format-version 1 --no-deps` -> `target_directory`, run with the project as cwd) and the contract must allow the resolved path to be (a) inside the project, (b) outside every project, (c) shared by N projects, then dedupe by resolved path. A shared directory has no sibling `Cargo.toml`, so a project walker never finds it. `cargo clean` in any one project wipes the shared directory for all of them.
3. **Per-project instances need a marker, and some per-workspace state lives outside the workspace.** Markers: sibling `Cargo.toml`, `CACHEDIR.TAG` (`unverified` for Cargo), `.buckconfig`, `build.zig`, `stack.yaml`, `mix.exs`, `shard.yml`, `CMakeCache.txt`. Bazel's output base is keyed by workspace path but stored under the user root, so it must be asked per workspace (`bazel info output_base`) and orphans of deleted workspaces cannot be found by walking projects; Nim puts per-project caches as children of one global directory.
4. **Read-only files defeat plain deletion.** Go's module cache is created read-only unless `GOFLAGS=-modcacherw`; the native `go clean -modcache` works either way. Contract needs `clean.native` preferred over `delete directory`, and a `needs_chmod_before_delete` flag for the fallback.
5. **A daemon or lock must be stopped first.** sccache (`sccache --stop-server`), Bazel (server per output base; `clean --expunge` stops it), Buck2 (`buck2 clean` kills the daemon, `--stale` does not), Conan 2 (cache not concurrent), gopls and rust-analyzer. Contract needs `pre_clean` (command) and `in_use_probe` (process/lock detection), plus a "native command handles it" marker.
6. **A directory that mixes cache with user data or configuration.** `$CARGO_HOME` (cache `registry/`, `git/` vs `bin/`, `config.toml`, `credentials.toml`), Conan home (cache vs profiles/remotes), `~/.config/rebar3`, `~/.mix/archives`, `~/.stack` (config.yaml, installed GHCs), opam root (switches), `~/.cabal` (config, `bin/`), `~/.nimble/bin`. A location must be an explicit list of subpaths (or a native command that prunes only the cache), never the root.
7. **Regenerable is a spectrum with a "user choice" class.** `yes`, `yes-but-slow-rebuild`, `yes-but-large-download`, and `no` (rustup toolchains/ghcup GHCs/opam switches/cargo-installed binaries/Conan packages created locally and never uploaded). A "no" entry must default to report-only, and require explicit per-item selection (e.g. one toolchain by name), not bulk clean.
8. **Multiple cleaning granularities per location.** Full wipe vs prune-unused vs partial scope: `go clean -cache|-testcache|-fuzzcache|-modcache`, `ccache -C|-c`, `cargo clean` with `-p|--release|--profile|--doc|--dry-run`, `buck2 clean --stale`, `opam clean` flag set, `stack clean|--full`, `cabal clean --save-config`, `dub clean|clean-caches|remove`. The contract needs a list of named actions per location, each with argv, effect description, and `dry_run` support (`cargo clean --dry-run`, `opam clean --dry-run`, `buck2 clean --dry-run`).
9. **Tool-managed garbage collection.** Cargo auto-GC (3-month/1-month thresholds, frequency 1 day), Go trims unused entries, sccache and ccache enforce a max size, Bazel 7.4+ disk-cache GC. The contract should record `self_pruning` and the config key, so the UI can say "already bounded to N".
10. **Tool present but no native clean, or the tool is absent while its directory exists.** sccache, vcpkg binary cache, Zig, Nim, CPM, rustup `downloads`, gopls have only `delete directory`; `detect` must therefore allow "directory exists" without the binary on PATH, and the registry must tell apart "no native command, deletion is the documented method" from "no command known (unverified)".
11. **Same binary, different major version or layout.** Conan 1 vs 2 (`~/.conan` vs `~/.conan2`, different commands); Cabal legacy `~/.cabal` vs XDG dirs; Mix opt-in `MIX_XDG`; Stack `STACK_XDG`; ccache legacy `~/.ccache` winning over XDG if present. `discover` must be version-aware or ask the tool.
12. **Defaults differ between macOS and Linux inconsistently.** Go, ccache, sccache and likely Zig use `~/Library/Caches/...` on macOS; vcpkg, Nim, rebar3 use `~/.cache/...` on macOS too. Defaults must be per-OS data, not derived from a rule.
13. **Location may be optional or remote.** sccache with a remote backend (nothing local), CPM (no default - exists only if `CPM_SOURCE_CACHE` is set), Bazel disk cache (opt-in, path in `.bazelrc`), `GOCACHEPROG`. The contract needs "not present" as a normal outcome, distinct from "discovery failed".
14. **Config-derived paths with relative-path semantics.** Cargo resolves a relative `build.target-dir` against the parent of the `.cargo` directory that defines it, but env/`--config` values against the cwd; a reimplementation must replicate this, which is why delegating to `cargo metadata` is preferred.
15. **Verification state must be carried in the registry.** Many entries above are `unverified`; the schema should hold per-field verification status so unverified clean commands are never executed by default (a wrong clean command in a public tool destroys data).
