package engine

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

const goCache = home + "/.cache/gotool"

// installedGoTool is a machine with the tool on PATH and 300 bytes in its cache.
func installedGoTool() *fake.System {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Out(goCache+"\n"))
	f.AddDir(goCache, 300)
	return f
}

func TestCleanRunsTheToolsCommandAndReportsTheDifference(t *testing.T) {
	f := installedGoTool()
	f.Script(argv("gotool clean"), fake.Result{Effect: func() { f.SetSize(goCache, 120) }})
	row := one(t, scan(t, f, goTool()))

	wantFreed(t, clean(t, f, row)[0], 180)
	if len(f.Removed()) != 0 {
		t.Fatalf("the directory was removed although the tool has a clean command: %v", f.Removed())
	}
}

func TestCleanReportsTheSizeBeforeWhenThePlaceIsGone(t *testing.T) {
	f := installedGoTool()
	f.Script(argv("gotool clean"), fake.Result{Effect: func() { f.Delete(goCache) }})
	row := one(t, scan(t, f, goTool()))

	wantFreed(t, clean(t, f, row)[0], 300)
}

func TestCleanRemovesTheDirectoryWhenThereIsNoCleanCommand(t *testing.T) {
	tool := goTool()
	tool.Places[0].Clean = entry.Clean{}
	f := installedGoTool()
	row := one(t, scan(t, f, tool))

	wantFreed(t, clean(t, f, row)[0], 300)
	if !slices.Equal(f.Removed(), []string{goCache}) {
		t.Fatalf("removal requested for %v, want exactly the directory of the row", f.Removed())
	}
}

func TestCleanRemovesTheDirectoryWhenTheExecutableIsGone(t *testing.T) {
	f := newFake()
	f.AddDir(goCache, 300)
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, goTool()))

	wantFreed(t, clean(t, f, row)[0], 300)
	if !slices.Equal(f.Removed(), []string{goCache}) {
		t.Fatalf("removal requested for %v", f.Removed())
	}
	if slices.Contains(ran(f), "gotool clean") {
		t.Fatal("the clean command was run although the executable is not on PATH")
	}
}

func TestUninstalledToolIsCleanedByRemovalWithoutItsCommands(t *testing.T) {
	tool := goTool()
	tool.Places[0].Requires = &entry.Requires{Command: "gotool ready"}
	tool.Places[0].Clean.Before = "gotool stop"
	f := newFake()
	f.AddDir(goCache, 300)
	f.Script(argv("gotool ready"), fake.Fail(1, "not ready"))
	f.Script(argv("gotool stop"), fake.Fail(1, "nothing to stop"))
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, tool))

	wantFreed(t, clean(t, f, row)[0], 300)
	if !slices.Equal(f.Removed(), []string{goCache}) {
		t.Fatalf("removed %v", f.Removed())
	}
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands of an uninstalled tool were run: %v", commands)
	}
}

func TestCleanFailsWhenTheToolDisappearedSinceTheScan(t *testing.T) {
	tool := goTool()
	tool.Places[0].Measure.Path = "<path>/download"
	f := installedGoTool()
	f.AddDir(goCache+"/download", 40)
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, tool))
	wantMeasured(t, row, goCache+"/download", 40)
	f.RemoveExecutable("gotool")

	wantFailed(t, clean(t, f, row)[0], "gotool is no longer on PATH: the tool disappeared since the scan")
	if len(f.Removed()) != 0 || slices.Contains(ran(f), "gotool clean") {
		t.Fatalf("removed %v, ran %v", f.Removed(), ran(f))
	}
}

func TestCleanFailsWhenAVersionRowsToolDisappearedSinceTheScan(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\n"))
	f.Script(argv("chain uninstall stable"), fake.Out(""))
	f.AddDir("/data/chain/toolchains/stable", 1000)
	row := one(t, scan(t, f, chainTool()))
	f.RemoveExecutable("chain")

	wantFailed(t, clean(t, f, row)[0], "the tool disappeared since the scan")
	if len(f.Removed()) != 0 || slices.Contains(ran(f), "chain uninstall stable") {
		t.Fatalf("removed %v, ran %v", f.Removed(), ran(f))
	}
}

func TestCleanFailsWhenTheToolAppearedSinceTheScan(t *testing.T) {
	f := newFake()
	f.AddDir(goCache, 300)
	f.Script(argv("gotool env CACHE"), fake.Out(goCache+"\n"))
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, goTool()))
	f.AddExecutable("gotool")

	wantFailed(t, clean(t, f, row)[0], "gotool is on PATH now and was not at the scan: scan again")
	if len(f.Removed()) != 0 || len(f.Ran()) != 0 {
		t.Fatalf("removed %v, ran %v", f.Removed(), ran(f))
	}
}

func TestCleanRefusesAVersionsRowOfAnInstalledToolThatNamesNoVersion(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\n"))
	f.Script([]string{"chain", "uninstall", ""}, fake.Out(""))
	f.SetStatError("/data/chain/toolchains", &fs.PathError{Op: "lstat", Path: "/data/chain/toolchains", Err: fs.ErrPermission})
	row := one(t, scan(t, f, chainTool()))
	wantState(t, row, NotMeasured, "permission denied")
	if row.Version != "" {
		t.Fatalf("the row names version %q", row.Version)
	}

	wantFailed(t, clean(t, f, row)[0], "the row names no version")
	if len(f.Removed()) != 0 || slices.Contains(ran(f), "chain uninstall ") {
		t.Fatalf("removed %v, ran %v", f.Removed(), ran(f))
	}
}

func TestCleanFailsWhenRequiresFails(t *testing.T) {
	tool := goTool()
	tool.Places[0].Requires = &entry.Requires{Command: "gotool ready"}
	tool.Places[0].Clean.Before = "gotool stop"
	f := installedGoTool()
	f.Script(argv("gotool ready"), fake.Out(""), fake.Fail(1, "the server is busy"))
	f.Script(argv("gotool stop"), fake.Out(""))
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, tool))

	wantFailed(t, clean(t, f, row)[0], "requires: `gotool ready`: exit status 1: the server is busy")
	if slices.Contains(ran(f), "gotool stop") || slices.Contains(ran(f), "gotool clean") || len(f.Removed()) != 0 {
		t.Fatalf("cleaning went on after requires failed: ran %v, removed %v", ran(f), f.Removed())
	}
}

func TestCleanRunsBeforeThenTheCommand(t *testing.T) {
	tool := goTool()
	tool.Places[0].Requires = &entry.Requires{Command: "gotool ready"}
	tool.Places[0].Clean.Before = "gotool stop --dir <path>"
	f := installedGoTool()
	f.Script(argv("gotool ready"), fake.Out(""))
	f.Script(argv("gotool stop --dir "+goCache), fake.Out(""))
	f.Script(argv("gotool clean"), fake.Result{Effect: func() { f.SetSize(goCache, 0) }})
	row := one(t, scan(t, f, tool))
	scanned := len(f.Ran())

	wantFreed(t, clean(t, f, row)[0], 300)
	want := []string{"gotool ready", "gotool stop --dir " + goCache, "gotool clean"}
	if got := ran(f)[scanned:]; !slices.Equal(got, want) {
		t.Fatalf("cleaning ran %v, want %v", got, want)
	}
}

func TestCleanFailsWhenBeforeFails(t *testing.T) {
	tool := goTool()
	tool.Places[0].Clean.Before = "gotool stop"
	f := installedGoTool()
	f.Script(argv("gotool stop"), fake.Fail(3, "no server to stop"))
	f.Script(argv("gotool clean"), fake.Out(""))
	row := one(t, scan(t, f, tool))

	wantFailed(t, clean(t, f, row)[0], "clean.before: `gotool stop`: exit status 3: no server to stop")
	if slices.Contains(ran(f), "gotool clean") || len(f.Removed()) != 0 {
		t.Fatalf("cleaning went on after clean.before failed: ran %v, removed %v", ran(f), f.Removed())
	}
}

func TestCleanFailsWithTheToolsMessage(t *testing.T) {
	cases := map[string]struct {
		result  fake.Result
		message string
	}{
		"exit status with a message": {fake.Fail(1, "cache is locked by another process\n"), "`gotool clean`: exit status 1: cache is locked by another process"},
		"message on standard output": {fake.Result{ExitCode: 2, Stdout: "nothing to do\n"}, "exit status 2: nothing to do"},
		"timeout":                    {fake.Hang(), "`gotool clean` timed out after 40ms"},
		"cannot start":               {fake.Result{Err: errors.New("permission denied")}, "`gotool clean`: permission denied"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := installedGoTool()
			f.Script(argv("gotool clean"), c.result)
			row := one(t, scan(t, f, goTool()))

			wantFailed(t, clean(t, f, row)[0], c.message)
			if len(f.Removed()) != 0 {
				t.Fatalf("the directory was removed after the tool's command failed: %v", f.Removed())
			}
		})
	}
}

func TestCleanFailsWhenRemovalIsRefused(t *testing.T) {
	f := newFake()
	f.AddDir(goCache, 300)
	f.SetRemoveError(goCache, errors.New(`refusing to remove "`+goCache+`": the path is a symbolic link`))
	row := one(t, scan(t, f, goTool()))

	wantFailed(t, clean(t, f, row)[0], "the path is a symbolic link")
}

func TestCleanFailsForAPlaceThatIsNotADirectoryWhenTheExecutableIsGone(t *testing.T) {
	tool := daemonTool()
	tool.Places[0].Requires = nil
	f := newFake()
	f.AddExecutable("daemon")
	f.Script(argv("daemon df"), fake.Out("Images 1GB\n"))
	f.Script(argv("daemon prune --force"), fake.Out(""))
	row := one(t, scan(t, f, tool))
	f.RemoveExecutable("daemon")

	wantFailed(t, clean(t, f, row)[0], "daemon is no longer on PATH: the tool disappeared since the scan")
	if slices.Contains(ran(f), "daemon prune --force") || len(f.Removed()) != 0 {
		t.Fatalf("ran %v, removed %v", ran(f), f.Removed())
	}
}

func TestCleanOfAPlaceThatIsNotADirectoryMeasuresAgain(t *testing.T) {
	f := newFake()
	f.AddExecutable("daemon")
	f.Script(argv("daemon info"), fake.Out(""))
	f.Script(argv("daemon df"), fake.Out("Images 1.5GB\n"), fake.Out("Images 200MB\n"))
	f.Script(argv("daemon prune --force"), fake.Out("Total reclaimed space: 1.3GB\n"))
	row := one(t, scan(t, f, daemonTool()))

	wantFreed(t, clean(t, f, row)[0], 1_300_000_000)
}

func TestCleanReportsNoNumberWhenTheSizeAfterIsNotKnown(t *testing.T) {
	f := newFake()
	f.AddExecutable("daemon")
	f.Script(argv("daemon info"), fake.Out(""))
	f.Script(argv("daemon df"), fake.Out("Images 1.5GB\n"), fake.Fail(1, "daemon went away"))
	f.Script(argv("daemon prune --force"), fake.Out(""))
	row := one(t, scan(t, f, daemonTool()))

	result := clean(t, f, row)[0]
	if result.Outcome != Cleaned || result.FreedKnown || result.Freed != 0 {
		t.Fatalf("got %+v", result)
	}
	if want := "the size after cleaning is not known: `daemon df`: exit status 1: daemon went away"; result.Message != want {
		t.Fatalf("message %q, want %q", result.Message, want)
	}
}

func TestCleanReportsNoNumberWhenTheSizeBeforeIsNotKnown(t *testing.T) {
	f := installedGoTool()
	f.AddDir(goCache+"/private", 50)
	f.SetUnreadable(goCache + "/private")
	f.Script(argv("gotool clean"), fake.Result{Effect: func() { f.Delete(goCache) }})
	row := one(t, scan(t, f, goTool()))
	wantState(t, row, NotMeasured, "permission denied")

	result := clean(t, f, row)[0]
	if result.Outcome != Cleaned || result.FreedKnown {
		t.Fatalf("got %+v", result)
	}
}

func TestCleanFailsForARowThatCouldNotBeDetermined(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Fail(1, "broken install"))
	f.Script(argv("gotool clean"), fake.Out(""))
	tool := goTool()
	tool.Places[0].Path.Default = nil
	row := one(t, scan(t, f, tool))
	wantState(t, row, Undetermined, "broken install")

	wantFailed(t, clean(t, f, row)[0], "the place could not be determined")
	if slices.Contains(ran(f), "gotool clean") || len(f.Removed()) != 0 {
		t.Fatalf("ran %v, removed %v", ran(f), f.Removed())
	}
}

func TestCleanRefusesToRemoveWhenThePlaceMovedSinceTheScan(t *testing.T) {
	tool := goTool()
	tool.Places[0].Clean = entry.Clean{}
	f := newFake()
	f.SetEnv("GOTOOL_CACHE", "/data/first")
	f.AddDir("/data/first", 300)
	f.AddDir("/data/second", 900)
	row := one(t, scan(t, f, tool))
	wantMeasured(t, row, "/data/first", 300)
	f.SetEnv("GOTOOL_CACHE", "/data/second")

	wantFailed(t, clean(t, f, row)[0], `scanned at "/data/first" and now resolves to "/data/second"`)
	if len(f.Removed()) != 0 {
		t.Fatalf("removal requested for %v", f.Removed())
	}
}

func TestCleanRefusesToRemoveWhenThePlaceNoLongerResolves(t *testing.T) {
	tool := goTool()
	tool.Places[0].Clean = entry.Clean{}
	tool.Places[0].Path.Default = nil
	f := newFake()
	f.SetEnv("GOTOOL_CACHE", "/data/first")
	f.AddDir("/data/first", 300)
	row := one(t, scan(t, f, tool))
	f.SetEnv("GOTOOL_CACHE", "")

	wantFailed(t, clean(t, f, row)[0], "no longer resolves to a path")
	if len(f.Removed()) != 0 {
		t.Fatalf("removal requested for %v", f.Removed())
	}
}

func TestAFailedRowDoesNotStopTheNext(t *testing.T) {
	second := goTool()
	second.Name = "second"
	second.Executable = "second"
	second.Places[0].Path = &entry.Path{Default: map[string]string{sys.Linux: "~/.cache/second"}}
	second.Places[0].Clean.Command = "second clean"

	f := installedGoTool()
	f.Script(argv("gotool clean"), fake.Fail(1, "cache is locked"))
	f.AddExecutable("second")
	f.AddDir(home+"/.cache/second", 70)
	f.Script(argv("second clean"), fake.Result{Effect: func() { f.Delete(home + "/.cache/second") }})
	f.AddDir(home+"/work/build", 40)
	rows := scan(t, f, goTool(), second, folder("build", "~/work/build"))
	if len(rows) != 3 {
		t.Fatalf("got %d rows", len(rows))
	}
	scanned := len(f.Ran())

	results := clean(t, f, rows...)
	wantFailed(t, results[0], "cache is locked")
	wantFreed(t, results[1], 70)
	wantFreed(t, results[2], 40)
	if got := ran(f)[scanned:]; !slices.Equal(got, []string{"gotool clean", "second clean"}) {
		t.Fatalf("cleaning ran %v", got)
	}
	if !slices.Equal(f.Removed(), []string{home + "/work/build"}) {
		t.Fatalf("removed %v", f.Removed())
	}
	for i, result := range results {
		if result.Row.Entry != rows[i].Entry {
			t.Fatalf("result %d is for %q, want %q: rows are not cleaned in order", i, result.Row.Entry, rows[i].Entry)
		}
	}
}

func TestRowsAreCleanedOneAfterAnother(t *testing.T) {
	second := goTool()
	second.Name = "second"
	second.Places[0].Name = "second cache"
	second.Places[0].Path = &entry.Path{Default: map[string]string{sys.Linux: "~/.cache/second"}}
	second.Places[0].Clean.Command = "gotool clean-second"

	f := installedGoTool()
	f.AddDir(home+"/.cache/second", 70)
	busy := false
	overlap := false
	effect := func() {
		if busy {
			overlap = true
		}
		busy = true
		time.Sleep(5 * time.Millisecond)
		busy = false
	}
	f.Script(argv("gotool clean"), fake.Result{Effect: effect})
	f.Script(argv("gotool clean-second"), fake.Result{Effect: effect})
	rows := scan(t, f, goTool(), second)

	settings := testSettings()
	settings.CleanTimeout = time.Minute
	e, err := New(f, settings)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range e.Clean(context.Background(), rows) {
		count++
	}
	if count != 2 {
		t.Fatalf("got %d results, want 2", count)
	}
	if overlap {
		t.Fatal("two rows were cleaned at the same time")
	}
}

func TestEverythingTakesCachePlacesAndFoldersButNoVersions(t *testing.T) {
	f := installedGoTool()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\nnightly\n"))
	f.AddDir("/data/chain/toolchains/stable", 1000)
	f.AddDir("/data/chain/toolchains/nightly", 2000)
	f.AddExecutable("daemon")
	f.Script(argv("daemon info"), fake.Fail(1, "daemon is not running"))
	f.AddDir(home+"/work/build", 40)
	f.Script(argv("gotool clean"), fake.Result{Effect: func() { f.Delete(goCache) }})
	f.Script(argv("chain uninstall stable"), fake.Out(""))
	f.Script(argv("chain uninstall nightly"), fake.Out(""))

	rows := scan(t, f, goTool(), chainTool(), daemonTool(), folder("build", "~/work/build"))
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5: %+v", len(rows), rows)
	}

	selected := Everything(rows)
	var names []string
	for _, row := range selected {
		names = append(names, row.Entry)
	}
	if !slices.Equal(names, []string{"gotool", "daemon", "build"}) {
		t.Fatalf("everything selected %v", names)
	}

	results := clean(t, f, selected...)
	wantFreed(t, results[0], 300)
	wantFailed(t, results[1], "daemon is not running")
	wantFreed(t, results[2], 40)
	for _, command := range ran(f) {
		if command == "chain uninstall stable" || command == "chain uninstall nightly" {
			t.Fatalf("clean everything ran %q", command)
		}
	}
	for _, removed := range f.Removed() {
		if removed != home+"/work/build" {
			t.Fatalf("clean everything removed %q", removed)
		}
	}
	for _, version := range []string{"stable", "nightly"} {
		if _, err := f.Stat("/data/chain/toolchains/" + version); err != nil {
			t.Fatalf("version %s is gone after clean everything: %v", version, err)
		}
	}
}

func TestAVersionRowIsCleanedWhenItIsChosen(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\nnightly\n"))
	f.AddDir("/data/chain/toolchains/stable", 1000)
	f.AddDir("/data/chain/toolchains/nightly", 2000)
	f.Script(argv("chain uninstall nightly"), fake.Result{Effect: func() { f.Delete("/data/chain/toolchains/nightly") }})
	rows := scan(t, f, chainTool())

	wantFreed(t, clean(t, f, rows[1])[0], 2000)
	if slices.Contains(ran(f), "chain uninstall stable") {
		t.Fatal("the version that was not chosen was uninstalled")
	}
	if _, err := f.Stat("/data/chain/toolchains/stable"); err != nil {
		t.Fatalf("the version that was not chosen is gone: %v", err)
	}
}

func TestAVersionWithoutACleanCommandIsRemovedAsItsOwnDirectory(t *testing.T) {
	tool := chainTool()
	tool.Places[0].Clean = entry.Clean{}
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\nnightly\n"))
	f.AddDir("/data/chain/toolchains/stable", 1000)
	f.AddDir("/data/chain/toolchains/nightly", 2000)
	rows := scan(t, f, tool)

	wantFreed(t, clean(t, f, rows[0])[0], 1000)
	if !slices.Equal(f.Removed(), []string{"/data/chain/toolchains/stable"}) {
		t.Fatalf("removed %v, want only the directory of the chosen version", f.Removed())
	}
}

func TestCleanStopsBeforeTheNextRowWhenTheContextEnds(t *testing.T) {
	f := installedGoTool()
	f.AddDir(home+"/work/build", 40)
	rows := scan(t, f, goTool(), folder("build", "~/work/build"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for result := range newEngine(t, f).Clean(ctx, rows) {
		t.Fatalf("a row was cleaned after the context ended: %+v", result)
	}
	if len(f.Removed()) != 0 {
		t.Fatalf("removed %v", f.Removed())
	}
}
