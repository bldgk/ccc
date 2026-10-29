// Command ccc shows how much disk space developer-tool caches take and cleans them.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/registry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/host"
	"github.com/bldgk/ccc/internal/ui"
)

const usage = `ccc shows how much disk space developer-tool caches take and cleans them.

Usage:
  ccc                              show the places and their sizes; mark rows and clean them
  ccc check                        the same
  ccc --clean                      clean every cache place and every folder, without confirmation
  ccc -c                           the same
  ccc add                          add a tool: a form asks how it is detected, measured and cleaned
  ccc add <path> [--name <name>]   add a directory as a folder
  ccc --help                       show this text
  ccc -h                           the same
`

const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2
)

// app is everything the program touches, so that a test can replace each part.
type app struct {
	system   sys.System
	settings engine.Settings
	load     func() (registry.Set, error)
	stdout   io.Writer
	stderr   io.Writer
	// terminal reports whether stdout is a terminal.
	terminal bool
	// inputTerminal reports whether stdin is a terminal.
	inputTerminal bool
	environ       []string
	screen        func(ui.Engine, []ui.Problem) error
	// form shows the form for adding a tool and returns the file it wrote, or an empty
	// path when the user left it.
	form func(ui.AddForm) (string, error)
	// workDir is the directory a relative path is relative to.
	workDir func() (string, error)
}

// bound is the engine together with the entries it works on.
type bound struct {
	engine  *engine.Engine
	entries []entry.Entry
}

func (b bound) Places() []engine.Place {
	return engine.Places(b.entries)
}

func (b bound) Scan(ctx context.Context) <-chan engine.PlaceResult {
	return b.engine.Scan(ctx, b.entries)
}

func (b bound) Clean(ctx context.Context, rows []engine.Row) <-chan engine.Result {
	return b.engine.Clean(ctx, rows)
}

func (b bound) Everything(rows []engine.Row) []engine.Row {
	return engine.Everything(rows)
}

func main() {
	os.Exit(app{
		system:        host.New(host.DefaultSettings()),
		settings:      engine.DefaultSettings(),
		load:          registry.Load,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		terminal:      term.IsTerminal(os.Stdout.Fd()),
		inputTerminal: term.IsTerminal(os.Stdin.Fd()),
		environ:       os.Environ(),
		screen:        ui.Run,
		form:          ui.RunAdd,
		workDir:       os.Getwd,
	}.run(os.Args[1:]))
}

func (a app) run(args []string) int {
	if len(args) > 0 && args[0] == "add" {
		return a.add(args[1:])
	}
	command := "check"
	if len(args) == 1 {
		command = args[0]
	}
	if len(args) > 1 {
		command = ""
	}
	switch command {
	case "check":
		return a.check()
	case "--clean", "-c":
		return a.clean()
	case "--help", "-h":
		fmt.Fprint(a.stdout, usage)
		return exitOK
	}
	fmt.Fprint(a.stderr, usage)
	return exitUsage
}

func (a app) prepare() (bound, []ui.Problem, error) {
	set, err := a.load()
	if err != nil {
		return bound{}, nil, err
	}
	eng, err := engine.New(a.system, a.settings)
	if err != nil {
		return bound{}, nil, err
	}
	problems := make([]ui.Problem, len(set.Problems))
	for i, p := range set.Problems {
		problems[i] = ui.Problem{Source: p.Source, Reason: p.Err.Error()}
	}
	return bound{engine: eng, entries: set.Entries}, problems, nil
}

func (a app) check() int {
	eng, problems, err := a.prepare()
	if err != nil {
		fmt.Fprintln(a.stderr, "ccc:", err)
		return exitFailed
	}
	if a.terminal {
		if err := a.screen(eng, problems); err != nil {
			fmt.Fprintln(a.stderr, "ccc:", err)
			return exitFailed
		}
		return exitOK
	}
	ui.WriteProblems(a.stderr, problems)
	ui.WriteRows(ui.Output(a.stdout, a.environ), ui.Collect(context.Background(), eng))
	return exitOK
}

func (a app) clean() int {
	eng, problems, err := a.prepare()
	if err != nil {
		fmt.Fprintln(a.stderr, "ccc:", err)
		return exitFailed
	}
	ui.WriteProblems(a.stderr, problems)
	totals := ui.CleanEverything(context.Background(), eng, ui.Output(a.stdout, a.environ))
	if totals.Failed > 0 {
		return exitFailed
	}
	return exitOK
}
