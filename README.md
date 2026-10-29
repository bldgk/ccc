# ccc — cute cache cleaner

`ccc` shows how much disk space the caches of developer tools take on your machine, and cleans
them. It knows each tool through a small text entry that says how to find the tool's cache, how
to measure it and how to clean it — with the tool's own command where the tool has one.

It runs on macOS and Linux.

## Install

Go 1.26 or newer:

    go install github.com/bldgk/ccc/cmd/ccc@latest

This puts `ccc` into `$(go env GOPATH)/bin`, which must be on your `PATH`.

From a clone:

    git clone https://github.com/bldgk/ccc.git
    cd ccc
    make install

`make install` puts the `ccc` binary into `~/.local/bin`; `make install BINDIR=/other/dir`
puts it elsewhere. That directory must be on your `PATH`. The shipped entries are built into the
binary, so it needs nothing else from the repository. `make uninstall` removes it again, and
`make build` builds `bin/ccc` without installing it.

## Use

    ccc            show the places and their sizes; mark rows and clean them
    ccc check      the same
    ccc --clean    clean every cache place and every folder, without confirmation
    ccc -c         the same
    ccc add        add a tool through a form
    ccc add PATH   add a folder; --name NAME gives the entry another name
    ccc --help     show the usage

On the screen: arrows or `j`/`k` move, space marks a row, `a` marks every cache, Enter asks for
confirmation and `y` cleans the marked rows, `q` quits.

Installed toolchain versions are listed one row per version. They are never part of "clean
everything": a version is removed only when you mark its row.

`ccc check | cat` prints the rows as plain text and removes nothing.

## What it knows

The shipped entries are the files in [`registry/`](registry/): Cargo, Docker, Go, Homebrew,
npm, NuGet, pip, pnpm, rustup, sccache, uv. An entry is shipped only after its detection and
its space check were run on a machine that has the tool.

`ccc` does not walk your projects. A directory inside a project — a `node_modules`, a `.next`,
a build output — is cleaned only when you added it with `ccc add PATH`.

## Your own entries

Your entries are TOML files in `$XDG_CONFIG_HOME/ccc/registry/`, or `~/.config/ccc/registry/`
when that variable is not set. `ccc add` writes them for you; you can also write one by hand.
A file with the name of a shipped entry replaces that entry.

    kind = "tool"
    name = "go"
    executable = "go"

    [[places]]
    name = "module cache"
    holds = "cache"
    path.command = "go env GOMODCACHE"
    path.env = "GOMODCACHE"
    path.default.linux = "~/go/pkg/mod"
    measure.by = "disk-usage"
    clean.command = "go clean -modcache"

A place without `clean.command` is cleaned by removing its directory. Every key is described
in [docs/spec/05-architecture.md](docs/spec/05-architecture.md#the-entry-file). A file with a
key `ccc` does not know is rejected with the reason, and the other entries still load.

## What it will not remove

`ccc` refuses to remove the root, your home directory, a directory that contains it, a path
fewer than two levels below the root, and a symbolic link. It never asks for administrator
rights. Commands of an entry run directly, without a shell.

## Documents

The requirements, the architecture and the order of building are in
[docs/spec/INDEX.md](docs/spec/INDEX.md).

## License

MIT — see [LICENSE](LICENSE).
