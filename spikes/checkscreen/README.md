# checkscreen — spike of the `ccc check` screen

A throwaway prototype of one terminal screen of `ccc`. It answers two questions:

1. Do the Charm libraries (Bubble Tea, Lip Gloss, Bubbles) do what the project assumes: a styled,
   selectable list that updates live while many external commands run in parallel, with a hung
   command cut off by a timeout.
2. What could "fun to look at and use" look like, so there is a real screen to react to.

It is not the product. The table of tools is hardcoded in `places.go` because this is a spike.

## Run

    go run .                    # interactive screen
    go run . --plain            # aligned text after all measurements finish (also the default when stdout is not a terminal)
    go run . --snapshot         # final screen rendered once as text, 80 columns, all cache rows marked
    go run . --demo-hang        # adds a fake entry whose command outlives the timeout
    go run . --timeout 5s       # per-command and per-measurement timeout (default 10s)

Keys: arrows or j/k move, space marks a row, `a` marks/unmarks all cache rows (never version
rows), Enter shows the marked rows in a panel, any key returns, q or Ctrl-C quits.
`NO_COLOR` is honoured by Bubble Tea's output layer.

## What it deliberately does not do

- It does not remove anything. There is no deleting code and no call to any tool's clean, prune,
  purge, gc or uninstall command. Enter only shows a panel.
- It does not read a registry file.
- It explains nothing about what a cache is for; sizes only.

Every place ends in one state: a size, "not present", "could not be determined" (path resolution
failed or timed out), or "not measured" (for example the Docker daemon is down or a measurement
timed out). Sizes are allocated blocks, hard links counted once; `≥` marks a lower bound when
some subdirectories were unreadable.

## Findings

Versions: Bubble Tea v2.1.0 (`charm.land/bubbletea/v2`), Lip Gloss v2.0.6
(`charm.land/lipgloss/v2`), Bubbles v2.2.1 (`charm.land/bubbles/v2`), Go 1.26. The v2 modules live
under the `charm.land` import path, not `github.com/charmbracelet`; most older tutorials are v1.

Worked out of the box:
- Live updates: a goroutine per place pushes results into one channel; a single `tea.Cmd` that
  reads the channel and re-arms itself after every message is enough. Rows fill in as results
  arrive and the spinner (`bubbles/spinner`) runs on the rest. The same channel is drained
  without Bubble Tea for `--plain` and `--snapshot`, so the engine is UI-independent.
- Styling: Lip Gloss styles, true-colour gradients (`Blend1D`) for the title and bars, rounded
  borders for the panel. Colour downsampling and `NO_COLOR` are handled by Bubble Tea when it
  writes.
- v2 API is small: `View()` returns `tea.View` (alt screen and window title are fields), keys
  arrive as `tea.KeyPressMsg` and the space bar's string is `"space"`.

Needed manual work:
- The timeout is not a library feature. It is `exec.CommandContext` plus a process group
  (`Setpgid`, kill the group on cancel, `WaitDelay`), so that child processes of the
  command die with it. Directory walking checks the context itself.
- The `bubbles/list` component was not used: rows here have five states, group headers, a
  non-selectable "pending" state and per-row bars, which is easier to draw by hand than to fit
  into its item/delegate model. Scrolling, cursor clamping and the footer are hand-written
  (about 40 lines). Whether `bubbles/list` or `viewport` would be worth adopting for the product
  was not evaluated beyond this.
- Plain/snapshot output needs its own path (`ansi.Strip` of the same render is used for the
  snapshot).

Limits and observations:
- Palette is fixed and tuned for dark backgrounds; light-background adaptation was not done.
- The interactive loop was exercised through a pseudo-terminal script (keys, Enter panel, quit),
  not by hand on several terminal emulators.
- The Linux defaults are coded (XDG paths) but were not run on Linux.
- `bun pm cache` only works inside a project directory, so its failure is treated as
  uninformative and the env/default path is used.
- Docker's two rows (images, build cache) come from one `docker system df` call each; sizes are
  decimal units as Docker prints them, converted to the same display unit as the rest.
