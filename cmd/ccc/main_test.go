package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/registry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
	"github.com/bldgk/ccc/internal/ui"
)

const home = "/home/alice"

func cacheTool(name string) entry.Entry {
	return entry.Entry{Kind: entry.Tool, Name: name, Executable: name, Places: []entry.Place{{
		Name:    "cache",
		Holds:   entry.Cache,
		Path:    &entry.Path{Default: map[string]string{sys.Linux: "~/.cache/" + name}},
		Measure: entry.Measure{By: entry.DiskUsage},
		Clean:   entry.Clean{Command: name + " clean"},
	}}}
}

type run struct {
	fake    *fake.System
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	screens int
	// forms are the forms for adding a tool that were opened.
	forms []ui.AddForm
	// formResult is what an opened form returns.
	formResult string
	app        app
}

// newRun is a machine with one tool whose cache of 1500 bytes cleans, and a folder of
// 700 bytes.
func newRun(t *testing.T) *run {
	t.Helper()
	f := fake.New(sys.Linux, home)
	f.AddExecutable("alpha")
	f.AddDir(home+"/.cache/alpha", 1500)
	f.Script([]string{"alpha", "clean"}, fake.Result{Effect: func() { f.Delete(home + "/.cache/alpha") }})
	f.AddDir("/work/scratch", 700)

	r := &run{fake: f}
	set := registry.Set{Entries: []entry.Entry{
		cacheTool("alpha"),
		{Kind: entry.Folder, Name: "scratch", Path: "/work/scratch"},
	}}
	for _, e := range set.Entries {
		if err := entry.Validate(e); err != nil {
			t.Fatalf("the test's entry %q is not valid: %v", e.Name, err)
		}
	}
	r.app = app{
		system: f,
		settings: engine.Settings{
			CommandTimeout: time.Second,
			WalkTimeout:    time.Second,
			CleanTimeout:   time.Second,
			Parallel:       4,
		},
		load:    func() (registry.Set, error) { return set, nil },
		stdout:  &r.stdout,
		stderr:  &r.stderr,
		environ: []string{},
		screen: func(ui.Engine, []ui.Problem) error {
			r.screens++
			return nil
		},
		form: func(form ui.AddForm) (string, error) {
			r.forms = append(r.forms, form)
			return r.formResult, nil
		},
		workDir: func() (string, error) { return home + "/project", nil },
	}
	return r
}

func (r *run) cleaned() bool {
	return len(r.fake.Removed()) > 0 || slices.ContainsFunc(r.fake.Ran(), func(argv []string) bool {
		return slices.Contains(argv, "clean")
	})
}

func TestCleanExitsZeroWhenNoRowFailed(t *testing.T) {
	for _, flag := range []string{"--clean", "-c"} {
		r := newRun(t)
		if status := r.app.run([]string{flag}); status != 0 {
			t.Fatalf("%s: exit status %d, want 0\n%s", flag, status, r.stdout.String())
		}
		want := "alpha    cache          freed 1.5 kB\nscratch  /work/scratch  freed 700 B\ntotal freed 2.2 kB, 0 failed\n"
		if got := r.stdout.String(); got != want {
			t.Fatalf("%s printed\n%s\nwant\n%s", flag, got, want)
		}
		if r.screens != 0 {
			t.Fatalf("%s opened the screen", flag)
		}
	}
}

func TestCleanExitsOneWhenARowFailed(t *testing.T) {
	r := newRun(t)
	r.fake.Script([]string{"alpha", "clean"}, fake.Fail(1, "boom"))
	if status := r.app.run([]string{"-c"}); status != 1 {
		t.Fatalf("exit status %d, want 1\n%s", status, r.stdout.String())
	}
	out := r.stdout.String()
	for _, want := range []string{"failed: `alpha clean`: exit status 1: boom", "scratch  /work/scratch  freed 700 B", "total freed 700 B, 1 failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the output lacks %q:\n%s", want, out)
		}
	}
}

func TestCleanedWithUnknownSizeIsNotAFailure(t *testing.T) {
	r := newRun(t)
	r.fake.AddDir("/work/scratch/locked", 1)
	r.fake.SetUnreadable("/work/scratch/locked")
	if status := r.app.run([]string{"-c"}); status != 0 {
		t.Fatalf("exit status %d, want 0\n%s", status, r.stdout.String())
	}
	if out := r.stdout.String(); !strings.Contains(out, "total freed 1.5 kB, 0 failed, 1 with freed size unknown") {
		t.Fatalf("the totals do not name the row without a freed size:\n%s", out)
	}
}

func TestCheckIntoAPipeListsAndRemovesNothing(t *testing.T) {
	for _, args := range [][]string{nil, {"check"}} {
		r := newRun(t)
		problem := registry.Problem{Source: "/etc/registry/odd.toml", Err: errors.New("unknown key: colour")}
		set, _ := r.app.load()
		set.Problems = []registry.Problem{problem}
		r.app.load = func() (registry.Set, error) { return set, nil }

		if status := r.app.run(args); status != 0 {
			t.Fatalf("%q: exit status %d, want 0", args, status)
		}
		// The version column is empty: neither row is an installed version.
		want := "alpha    cache            1.5 kB\nscratch  /work/scratch    700 B\n"
		if got := r.stdout.String(); got != want {
			t.Fatalf("%q printed\n%s\nwant\n%s", args, got, want)
		}
		if got, want := r.stderr.String(), "/etc/registry/odd.toml: unknown key: colour\n"; got != want {
			t.Fatalf("%q: stderr %q, want %q", args, got, want)
		}
		if r.cleaned() || r.screens != 0 {
			t.Fatalf("%q: cleaned %v, screens %d; want a listing only", args, r.cleaned(), r.screens)
		}
	}
}

func TestCheckOnATerminalOpensTheScreen(t *testing.T) {
	for _, args := range [][]string{nil, {"check"}} {
		r := newRun(t)
		r.app.terminal = true
		if status := r.app.run(args); status != 0 || r.screens != 1 {
			t.Fatalf("%q: exit status %d, screens %d; want 0 and the screen once", args, status, r.screens)
		}
		if r.stdout.Len() != 0 || r.cleaned() {
			t.Fatalf("%q: printed %q, cleaned %v", args, r.stdout.String(), r.cleaned())
		}
	}
}

func TestHelpPrintsUsage(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		r := newRun(t)
		if status := r.app.run([]string{flag}); status != 0 {
			t.Fatalf("%s: exit status %d, want 0", flag, status)
		}
		if r.stdout.String() != usage || r.stderr.Len() != 0 {
			t.Fatalf("%s: stdout %q, stderr %q", flag, r.stdout.String(), r.stderr.String())
		}
	}
}

func TestUnknownArgumentsPrintUsageOnStderrAndCleanNothing(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"-c", "extra"}, {"check", "-c"}, {""}} {
		r := newRun(t)
		r.app.terminal = true
		if status := r.app.run(args); status == 0 {
			t.Fatalf("%q: exit status 0, want non-zero", args)
		}
		if r.stderr.String() != usage || r.stdout.Len() != 0 {
			t.Fatalf("%q: stdout %q, stderr %q", args, r.stdout.String(), r.stderr.String())
		}
		if r.cleaned() || r.screens != 0 || len(r.fake.Ran()) != 0 {
			t.Fatalf("%q: something ran", args)
		}
	}
}

func TestInvalidShippedRegistryStops(t *testing.T) {
	r := newRun(t)
	r.app.load = func() (registry.Set, error) { return registry.Set{}, errors.New("shipped registry: bad") }
	if status := r.app.run([]string{"-c"}); status != 1 {
		t.Fatalf("exit status %d, want 1", status)
	}
	if r.cleaned() || !strings.Contains(r.stderr.String(), "shipped registry: bad") {
		t.Fatalf("cleaned %v, stderr %q", r.cleaned(), r.stderr.String())
	}
}
