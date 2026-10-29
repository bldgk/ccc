package engine

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

const home = "/home/alice"

func testSettings() Settings {
	return Settings{
		CommandTimeout: 40 * time.Millisecond,
		WalkTimeout:    40 * time.Millisecond,
		CleanTimeout:   40 * time.Millisecond,
		Parallel:       4,
	}
}

func newFake() *fake.System {
	return fake.New(sys.Linux, home)
}

func newEngine(t *testing.T, f *fake.System) *Engine {
	t.Helper()
	e, err := New(f, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func argv(command string) []string {
	return strings.Fields(command)
}

func scan(t *testing.T, f *fake.System, entries ...entry.Entry) []Row {
	t.Helper()
	for _, e := range entries {
		if err := entry.Validate(e); err != nil {
			t.Fatalf("the test's entry %q is not valid: %v", e.Name, err)
		}
	}
	rows := flatten(t, entries, results(newEngine(t, f), entries))
	slices.SortFunc(rows, func(a, b Row) int {
		for _, d := range []int{a.EntryIndex - b.EntryIndex, a.PlaceIndex - b.PlaceIndex, a.VersionIndex - b.VersionIndex} {
			if d != 0 {
				return d
			}
		}
		return 0
	})
	return rows
}

func results(e *Engine, entries []entry.Entry) []PlaceResult {
	var all []PlaceResult
	for result := range e.Scan(context.Background(), entries) {
		all = append(all, result)
	}
	return all
}

// flatten returns the rows of a finished scan, after checking that every planned place
// got exactly one result and that every row lies in the result of its own place.
func flatten(t *testing.T, entries []entry.Entry, all []PlaceResult) []Row {
	t.Helper()
	planned := Places(entries)
	seen := map[Place]int{}
	var rows []Row
	for _, result := range all {
		seen[result.Place]++
		for _, row := range result.Rows {
			if row.Entry != result.Place.Entry || row.Place != result.Place.Name ||
				row.EntryIndex != result.Place.EntryIndex || row.PlaceIndex != result.Place.PlaceIndex {
				t.Fatalf("the result of place %+v holds the row %+v", result.Place, row)
			}
		}
		rows = append(rows, result.Rows...)
	}
	if len(all) != len(planned) {
		t.Fatalf("got %d results for %d planned places", len(all), len(planned))
	}
	for _, place := range planned {
		if seen[place] != 1 {
			t.Fatalf("the planned place %+v got %d results", place, seen[place])
		}
	}
	return rows
}

func one(t *testing.T, rows []Row) Row {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want one: %+v", len(rows), rows)
	}
	return rows[0]
}

func clean(t *testing.T, f *fake.System, rows ...Row) []Result {
	t.Helper()
	var results []Result
	for result := range newEngine(t, f).Clean(context.Background(), rows) {
		results = append(results, result)
	}
	if len(results) != len(rows) {
		t.Fatalf("got %d results for %d rows", len(results), len(rows))
	}
	return results
}

func ran(f *fake.System) []string {
	var commands []string
	for _, c := range f.Ran() {
		commands = append(commands, strings.Join(c, " "))
	}
	return commands
}

func wantMeasured(t *testing.T, row Row, dir string, size int64) {
	t.Helper()
	if row.State != Measured || row.Dir != dir || row.Size != size {
		t.Fatalf("got state %q, dir %q, size %d, reason %q; want measured %q with %d bytes",
			row.State, row.Dir, row.Size, row.Reason, dir, size)
	}
}

func wantState(t *testing.T, row Row, state State, reason string) {
	t.Helper()
	if row.State != state || !strings.Contains(row.Reason, reason) {
		t.Fatalf("got state %q with reason %q; want %q with a reason containing %q", row.State, row.Reason, state, reason)
	}
}

func wantFailed(t *testing.T, result Result, message string) {
	t.Helper()
	if result.Outcome != Failed || !strings.Contains(result.Message, message) {
		t.Fatalf("got %q with message %q; want failed with a message containing %q", result.Outcome, result.Message, message)
	}
}

func wantFreed(t *testing.T, result Result, freed int64) {
	t.Helper()
	if result.Outcome != Cleaned || !result.FreedKnown || result.Freed != freed {
		t.Fatalf("got %q, freed %d (known: %v), message %q; want cleaned with %d freed",
			result.Outcome, result.Freed, result.FreedKnown, result.Message, freed)
	}
}

// goTool is a tool whose cache is a directory found through all three sources.
func goTool() entry.Entry {
	return entry.Entry{Kind: entry.Tool, Name: "gotool", Executable: "gotool", Places: []entry.Place{{
		Name:  "cache",
		Holds: entry.Cache,
		Path: &entry.Path{
			Command: "gotool env CACHE",
			Env:     "GOTOOL_CACHE",
			Default: map[string]string{sys.Linux: "~/.cache/gotool", sys.MacOS: "~/Library/Caches/gotool"},
		},
		Measure: entry.Measure{By: entry.DiskUsage},
		Clean:   entry.Clean{Command: "gotool clean"},
	}}}
}

// daemonTool is a tool whose place is not a directory.
func daemonTool() entry.Entry {
	return entry.Entry{Kind: entry.Tool, Name: "daemon", Executable: "daemon", Places: []entry.Place{{
		Name:     "images",
		Holds:    entry.Cache,
		Requires: &entry.Requires{Command: "daemon info"},
		Measure:  entry.Measure{By: entry.Command, Command: "daemon df", Pattern: `^Images (\S+)`},
		Clean:    entry.Clean{Command: "daemon prune --force"},
	}}}
}

// chainTool is a tool with a place of installed versions.
func chainTool() entry.Entry {
	return entry.Entry{Kind: entry.Tool, Name: "chain", Executable: "chain", Places: []entry.Place{{
		Name:    "toolchains",
		Holds:   entry.Versions,
		Path:    &entry.Path{Command: "chain home", Subdir: "toolchains", Default: map[string]string{sys.Linux: "~/.chain/toolchains"}},
		List:    &entry.List{Command: "chain list", Pattern: `^(\S+)`},
		Measure: entry.Measure{By: entry.DiskUsage, Path: "<path>/<version>"},
		Clean:   entry.Clean{Command: "chain uninstall <version>"},
	}}}
}

func folder(name, path string) entry.Entry {
	return entry.Entry{Kind: entry.Folder, Name: name, Path: path}
}
