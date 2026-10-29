package engine

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

func TestPathCommandWinsOverTheOtherSources(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Out("/data/from-command\nsecond line\n"))
	f.SetEnv("GOTOOL_CACHE", "/data/from-env")
	f.AddDir("/data/from-command", 100)
	f.AddDir("/data/from-env", 200)
	f.AddDir(home+"/.cache/gotool", 300)

	wantMeasured(t, one(t, scan(t, f, goTool())), "/data/from-command", 100)
}

func TestPathFallsThroughToTheEnvironmentVariable(t *testing.T) {
	results := map[string]fake.Result{
		"the command fails":                  fake.Fail(1, "no such setting"),
		"the command prints nothing":         fake.Out("  \n"),
		"the command cannot start":           {Err: errors.New("exec format error")},
		"the command hangs":                  fake.Hang(),
		"the command prints a relative path": fake.Out("cache/dir\n"),
	}
	for name, result := range results {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.AddExecutable("gotool")
			f.Script(argv("gotool env CACHE"), result)
			f.SetEnv("GOTOOL_CACHE", "/data/from-env")
			f.AddDir("/data/from-env", 200)
			f.AddDir(home+"/.cache/gotool", 300)

			wantMeasured(t, one(t, scan(t, f, goTool())), "/data/from-env", 200)
		})
	}
}

func TestPathCommandIsNotRunWhenTheExecutableIsAbsent(t *testing.T) {
	f := newFake()
	f.Script(argv("gotool env CACHE"), fake.Out("/data/from-command\n"))
	f.SetEnv("GOTOOL_CACHE", "/data/from-env")
	f.AddDir("/data/from-command", 100)
	f.AddDir("/data/from-env", 200)

	wantMeasured(t, one(t, scan(t, f, goTool())), "/data/from-env", 200)
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands were run without the executable: %v", commands)
	}
}

func TestPathFallsThroughToTheDefaultOfTheOperatingSystem(t *testing.T) {
	cases := map[string]struct {
		os       string
		env      map[string]string
		wantPath string
	}{
		"linux, variable not set":        {sys.Linux, nil, home + "/.cache/gotool"},
		"macos, variable not set":        {sys.MacOS, nil, home + "/Library/Caches/gotool"},
		"variable set but empty":         {sys.Linux, map[string]string{"GOTOOL_CACHE": ""}, home + "/.cache/gotool"},
		"variable holds a relative path": {sys.Linux, map[string]string{"GOTOOL_CACHE": "relative"}, home + "/.cache/gotool"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := fake.New(c.os, home)
			f.AddExecutable("gotool")
			f.Script(argv("gotool env CACHE"), fake.Fail(1, ""))
			for k, v := range c.env {
				f.SetEnv(k, v)
			}
			f.AddDir(home+"/.cache/gotool", 300)
			f.AddDir(home+"/Library/Caches/gotool", 400)

			row := one(t, scan(t, f, goTool()))
			if row.State != Measured || row.Dir != c.wantPath {
				t.Fatalf("got %q at %q (%s)", row.State, row.Dir, row.Reason)
			}
		})
	}
}

func TestNoSourceWithTheExecutableOnPathCouldNotBeDetermined(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Fail(2, "unknown variable CACHE"))
	tool := goTool()
	delete(tool.Places[0].Path.Default, sys.Linux)

	row := one(t, scan(t, f, tool))
	wantState(t, row, Undetermined, "unknown variable CACHE")
	for _, part := range []string{"path.command", "path.env: GOTOOL_CACHE is not set", "no default for linux"} {
		if !strings.Contains(row.Reason, part) {
			t.Errorf("the reason %q does not mention %q", row.Reason, part)
		}
	}
	if row.Dir != "" || row.Size != 0 {
		t.Errorf("an undetermined row carries dir %q and size %d", row.Dir, row.Size)
	}
}

func TestNoSourceAfterATimeoutCouldNotBeDetermined(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Hang())
	tool := goTool()
	tool.Places[0].Path.Env = ""
	tool.Places[0].Path.Default = nil

	wantState(t, one(t, scan(t, f, tool)), Undetermined, "timed out after 40ms")
}

func TestNoSourceWithoutTheExecutableIsAbsent(t *testing.T) {
	f := newFake()
	tool := goTool()
	delete(tool.Places[0].Path.Default, sys.Linux)

	if rows := scan(t, f, tool); len(rows) != 0 {
		t.Fatalf("got rows for a tool that is not here: %+v", rows)
	}
}

func TestResolvedDirectoryThatDoesNotExistIsAbsent(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Out("/data/never-created\n"))
	f.AddDir(home+"/.cache/gotool", 300)

	if rows := scan(t, f, goTool()); len(rows) != 0 {
		t.Fatalf("got rows: %+v", rows)
	}
}

func TestResolvedPathThatIsAFileIsAbsent(t *testing.T) {
	f := newFake()
	f.SetEnv("GOTOOL_CACHE", "/data/file")
	f.AddFile("/data/file", 10)

	if rows := scan(t, f, goTool()); len(rows) != 0 {
		t.Fatalf("got rows: %+v", rows)
	}
}

func TestPathThatCannotBeLookedAtIsNotMeasured(t *testing.T) {
	f := newFake()
	f.SetEnv("GOTOOL_CACHE", "/data/cache")
	f.SetStatError("/data/cache", &fs.PathError{Op: "lstat", Path: "/data/cache", Err: fs.ErrPermission})

	row := one(t, scan(t, f, goTool()))
	wantState(t, row, NotMeasured, "permission denied")
	if row.Path != "/data/cache" || row.Dir != "" {
		t.Fatalf("got path %q and dir %q; want the resolved path and no directory", row.Path, row.Dir)
	}
}

func TestCacheLeftBehindByAnUninstalledToolIsMeasured(t *testing.T) {
	f := newFake()
	f.AddDir(home+"/.cache/gotool", 300)

	wantMeasured(t, one(t, scan(t, f, goTool())), home+"/.cache/gotool", 300)
}

func TestSubdirIsAppendedToCommandAndEnvButNotToADefault(t *testing.T) {
	tool := goTool()
	tool.Places[0].Path.Subdir = "objects/v1"
	tool.Places[0].Path.Default[sys.Linux] = "~/.cache/gotool-default"

	t.Run("command", func(t *testing.T) {
		f := newFake()
		f.AddExecutable("gotool")
		f.Script(argv("gotool env CACHE"), fake.Out("/data/home/\n"))
		f.AddDir("/data/home", 1)
		f.AddDir("/data/home/objects/v1", 50)
		wantMeasured(t, one(t, scan(t, f, tool)), "/data/home/objects/v1", 50)
	})
	t.Run("env", func(t *testing.T) {
		f := newFake()
		f.SetEnv("GOTOOL_CACHE", "/data/env")
		f.AddDir("/data/env/objects/v1", 60)
		wantMeasured(t, one(t, scan(t, f, tool)), "/data/env/objects/v1", 60)
	})
	t.Run("default", func(t *testing.T) {
		f := newFake()
		f.AddDir(home+"/.cache/gotool-default", 70)
		f.AddDir(home+"/.cache/gotool-default/objects/v1", 5)
		wantMeasured(t, one(t, scan(t, f, tool)), home+"/.cache/gotool-default", 75)
	})
}

func TestExpansionInDefaultsAndFolderPaths(t *testing.T) {
	cases := []struct {
		text string
		env  map[string]string
		want string
	}{
		{"~", nil, home},
		{"~/.cache/x", nil, home + "/.cache/x"},
		{"/var/cache/x", nil, "/var/cache/x"},
		{"$XDG_CACHE_HOME/x", map[string]string{"XDG_CACHE_HOME": "/data/cache"}, "/data/cache/x"},
		{"${XDG_CACHE_HOME}/x", map[string]string{"XDG_CACHE_HOME": "/data/cache"}, "/data/cache/x"},
		{"${ROOT}_old/x", map[string]string{"ROOT": "/data"}, "/data_old/x"},
		{"$ROOT/$NAME-$NAME", map[string]string{"ROOT": "/data", "NAME": "n1"}, "/data/n1-n1"},
		{"~/a/$NAME/", map[string]string{"NAME": "n1"}, home + "/a/n1"},
		{"/data/a~b", nil, "/data/a~b"},
		{"/data/cost$", nil, "/data/cost$"},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			f := newFake()
			for k, v := range c.env {
				f.SetEnv(k, v)
			}
			got, err := newEngine(t, f).expand(c.text)
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestExpansionGivesNothing(t *testing.T) {
	cases := map[string]struct {
		text   string
		env    map[string]string
		reason string
	}{
		"variable not set":           {"$XDG_CACHE_HOME/x", nil, "XDG_CACHE_HOME is not set"},
		"braced variable not set":    {"${XDG_CACHE_HOME}/x", nil, "XDG_CACHE_HOME is not set"},
		"variable set to nothing":    {"$XDG_CACHE_HOME/x", map[string]string{"XDG_CACHE_HOME": ""}, "XDG_CACHE_HOME is not set"},
		"one of two variables unset": {"$A/$B", map[string]string{"A": "/data"}, "B is not set"},
		"brace not closed":           {"${A/x", map[string]string{"A": "/data"}, "not closed"},
		"result is not absolute":     {"$A/x", map[string]string{"A": "data"}, "not an absolute path"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			for k, v := range c.env {
				f.SetEnv(k, v)
			}
			got, err := newEngine(t, f).expand(c.text)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %q, %v; want an error containing %q", got, err, c.reason)
			}
		})
	}

	t.Run("home directory unknown", func(t *testing.T) {
		f := newFake()
		f.SetHomeError(errors.New("no home"))
		if got, err := newEngine(t, f).expand("~/x"); err == nil {
			t.Fatalf("got %q", got)
		}
	})
}

func TestDefaultWithAnUnsetVariableGivesNothing(t *testing.T) {
	tool := goTool()
	tool.Places[0].Path.Default[sys.Linux] = "$XDG_CACHE_HOME/gotool"

	t.Run("set", func(t *testing.T) {
		f := newFake()
		f.SetEnv("XDG_CACHE_HOME", "/data/xdg")
		f.AddDir("/data/xdg/gotool", 10)
		wantMeasured(t, one(t, scan(t, f, tool)), "/data/xdg/gotool", 10)
	})
	t.Run("not set, executable absent", func(t *testing.T) {
		f := newFake()
		f.AddDir("/gotool", 10)
		if rows := scan(t, f, tool); len(rows) != 0 {
			t.Fatalf("got rows: %+v", rows)
		}
	})
	t.Run("not set, executable on PATH", func(t *testing.T) {
		f := newFake()
		f.AddExecutable("gotool")
		f.Script(argv("gotool env CACHE"), fake.Fail(1, ""))
		f.AddDir("/gotool", 10)
		wantState(t, one(t, scan(t, f, tool)), Undetermined, "XDG_CACHE_HOME is not set")
	})
}

func TestFolder(t *testing.T) {
	t.Run("measured by disk usage", func(t *testing.T) {
		f := newFake()
		f.AddDir(home+"/work/app/build", 500)
		f.AddDir(home+"/work/app/build/obj", 250)
		row := one(t, scan(t, f, folder("build", "~/work/app/build")))
		wantMeasured(t, row, home+"/work/app/build", 750)
		if row.Kind != entry.Folder || row.Holds != entry.Cache || row.Entry != "build" {
			t.Fatalf("row described as %+v", row)
		}
	})
	t.Run("under a variable", func(t *testing.T) {
		f := newFake()
		f.SetEnv("WORK", "/data/work")
		f.AddDir("/data/work/build", 20)
		wantMeasured(t, one(t, scan(t, f, folder("build", "${WORK}/build"))), "/data/work/build", 20)
	})
	t.Run("cannot be looked at", func(t *testing.T) {
		f := newFake()
		f.SetStatError(home+"/work/app/build", &fs.PathError{Op: "lstat", Path: home + "/work/app/build", Err: fs.ErrPermission})
		row := one(t, scan(t, f, folder("build", "~/work/app/build")))
		wantState(t, row, NotMeasured, "permission denied")
		if row.Path != home+"/work/app/build" {
			t.Fatalf("the folder row carries the path %q", row.Path)
		}
	})
	t.Run("variable not set", func(t *testing.T) {
		if rows := scan(t, newFake(), folder("build", "$WORK/build")); len(rows) != 0 {
			t.Fatalf("got rows: %+v", rows)
		}
	})
	t.Run("directory not there", func(t *testing.T) {
		if rows := scan(t, newFake(), folder("build", "~/work/app/build")); len(rows) != 0 {
			t.Fatalf("got rows: %+v", rows)
		}
	})
}

func TestPlaceThatIsNotADirectory(t *testing.T) {
	t.Run("executable absent", func(t *testing.T) {
		f := newFake()
		if rows := scan(t, f, daemonTool()); len(rows) != 0 {
			t.Fatalf("got rows: %+v", rows)
		}
		if commands := ran(f); len(commands) != 0 {
			t.Fatalf("commands were run: %v", commands)
		}
	})
	t.Run("measured by its command", func(t *testing.T) {
		f := newFake()
		f.AddExecutable("daemon")
		f.Script(argv("daemon info"), fake.Out("ok\n"))
		f.Script(argv("daemon df"), fake.Out("Containers 12kB\nImages 1.5GB (40%)\nBuild Cache 3MB\n"))
		row := one(t, scan(t, f, daemonTool()))
		wantMeasured(t, row, "", 1_500_000_000)
		if !slices.Equal(ran(f), []string{"daemon info", "daemon df"}) {
			t.Fatalf("commands ran as %v", ran(f))
		}
	})
}

func TestRequiresThatFailsGivesNotMeasured(t *testing.T) {
	f := newFake()
	f.AddExecutable("daemon")
	f.Script(argv("daemon info"), fake.Fail(1, "cannot connect to the daemon"))
	f.Script(argv("daemon df"), fake.Out("Images 1GB\n"))

	wantState(t, one(t, scan(t, f, daemonTool())), NotMeasured, "requires: `daemon info`: exit status 1: cannot connect to the daemon")
	if slices.Contains(ran(f), "daemon df") {
		t.Fatal("the place was measured although requires failed")
	}
}

func TestRequiresThatHangsGivesNotMeasured(t *testing.T) {
	f := newFake()
	f.AddExecutable("daemon")
	f.Script(argv("daemon info"), fake.Hang())

	wantState(t, one(t, scan(t, f, daemonTool())), NotMeasured, "timed out after 40ms")
}

func TestMeasuringByCommandThatFailsGivesNotMeasured(t *testing.T) {
	cases := map[string]struct {
		result fake.Result
		reason string
	}{
		"the command fails":            {fake.Fail(125, "df: permission denied"), "`daemon df`: exit status 125: df: permission denied"},
		"the command fails silently":   {fake.Fail(4, ""), "`daemon df`: exit status 4"},
		"the command hangs":            {fake.Hang(), "`daemon df` timed out after 40ms"},
		"the pattern does not match":   {fake.Out("Containers 12kB\n"), "no line of the output matches measure.pattern"},
		"the size does not parse":      {fake.Out("Images lots\n"), `"lots" is not a size`},
		"the size has an unknown unit": {fake.Out("Images 12parsecs\n"), `unknown unit "parsecs"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.AddExecutable("daemon")
			f.Script(argv("daemon info"), fake.Out(""))
			f.Script(argv("daemon df"), c.result)
			row := one(t, scan(t, f, daemonTool()))
			wantState(t, row, NotMeasured, c.reason)
			if row.Size != 0 {
				t.Fatalf("a row that is not measured carries the size %d", row.Size)
			}
		})
	}
}

func TestDiskUsageThatFailsGivesNotMeasured(t *testing.T) {
	t.Run("a part cannot be read", func(t *testing.T) {
		f := newFake()
		f.AddDir(home+"/.cache/gotool", 300)
		f.AddDir(home+"/.cache/gotool/private", 100)
		f.SetUnreadable(home + "/.cache/gotool/private")
		row := one(t, scan(t, f, goTool()))
		wantState(t, row, NotMeasured, "permission denied")
		if row.Size != 0 {
			t.Fatalf("a partial sum %d is shown", row.Size)
		}
		if row.Path != home+"/.cache/gotool" {
			t.Fatalf("a row that is not measured carries the path %q", row.Path)
		}
	})
	t.Run("the walk times out", func(t *testing.T) {
		f := newFake()
		f.AddDir(home+"/.cache/gotool", 300)
		f.HangDiskUsage(home + "/.cache/gotool")
		wantState(t, one(t, scan(t, f, goTool())), NotMeasured, "timed out after 40ms")
	})
	t.Run("the path is a symbolic link", func(t *testing.T) {
		f := newFake()
		f.AddSymlink(home + "/.cache/gotool")
		wantState(t, one(t, scan(t, f, goTool())), NotMeasured, "not a directory")
	})
}

func TestTimeoutsAreSettings(t *testing.T) {
	f := newFake()
	f.AddDir(home+"/.cache/gotool", 300)
	f.HangDiskUsage(home + "/.cache/gotool")
	settings := testSettings()
	settings.WalkTimeout = 15 * time.Millisecond
	settings.CommandTimeout = time.Hour
	e, err := New(f, settings)
	if err != nil {
		t.Fatal(err)
	}
	entries := []entry.Entry{goTool()}
	wantState(t, one(t, flatten(t, entries, results(e, entries))), NotMeasured, "timed out after 15ms")
}

func TestSettingsMustBePositive(t *testing.T) {
	if _, err := New(newFake(), DefaultSettings()); err != nil {
		t.Fatalf("the defaults were rejected: %v", err)
	}
	for _, change := range []func(*Settings){
		func(s *Settings) { s.CommandTimeout = 0 },
		func(s *Settings) { s.WalkTimeout = 0 },
		func(s *Settings) { s.CleanTimeout = -time.Second },
		func(s *Settings) { s.Parallel = 0 },
	} {
		settings := DefaultSettings()
		change(&settings)
		if _, err := New(newFake(), settings); err == nil {
			t.Errorf("accepted %+v", settings)
		}
	}
}

func TestVersionsPlaceGivesOneRowPerVersion(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable-x86 (default)\n\nnightly-2\nstable-x86 (override)\n"))
	f.AddDir("/data/chain/toolchains/stable-x86", 1000)
	f.AddDir("/data/chain/toolchains/nightly-2", 2000)

	rows := scan(t, f, chainTool())
	if len(rows) != 2 {
		t.Fatalf("got %d rows: %+v", len(rows), rows)
	}
	wantMeasured(t, rows[0], "/data/chain/toolchains/stable-x86", 1000)
	wantMeasured(t, rows[1], "/data/chain/toolchains/nightly-2", 2000)
	if rows[0].Version != "stable-x86" || rows[1].Version != "nightly-2" || rows[0].Holds != entry.Versions {
		t.Fatalf("rows described as %+v", rows)
	}
	if rows[0].Path != rows[0].Dir || rows[1].Path != rows[1].Dir {
		t.Fatalf("measured rows carry paths %q and %q, not their directories", rows[0].Path, rows[1].Path)
	}
}

func TestVersionsPlaceDeliversAllItsRowsInOneResult(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("one\ntwo\nthree\n"))
	for _, version := range []string{"one", "two", "three"} {
		f.AddDir("/data/chain/toolchains/"+version, 10)
	}

	all := results(newEngine(t, f), []entry.Entry{chainTool()})
	if len(all) != 1 || len(all[0].Rows) != 3 {
		t.Fatalf("got %+v; want one result with three rows", all)
	}
	for i, version := range []string{"one", "two", "three"} {
		if row := all[0].Rows[i]; row.Version != version || row.VersionIndex != i {
			t.Fatalf("row %d is version %q at index %d", i, row.Version, row.VersionIndex)
		}
	}
}

func TestPlacesListsWhatAScanLooksAt(t *testing.T) {
	two := goTool()
	two.Places = append(two.Places, daemonTool().Places[0])
	entries := []entry.Entry{two, folder("build", "~/work/build"), chainTool()}

	want := []Place{
		{Entry: "gotool", Name: "cache", Kind: entry.Tool, EntryIndex: 0, PlaceIndex: 0},
		{Entry: "gotool", Name: "images", Kind: entry.Tool, EntryIndex: 0, PlaceIndex: 1},
		{Entry: "build", Name: "", Kind: entry.Folder, Path: "~/work/build", EntryIndex: 1, PlaceIndex: 0},
		{Entry: "chain", Name: "toolchains", Kind: entry.Tool, EntryIndex: 2, PlaceIndex: 0},
	}
	if got := Places(entries); !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAbsentPlaceDeliversAResultWithoutRows(t *testing.T) {
	f := newFake()
	f.AddDir(home+"/work/build", 5)
	entries := []entry.Entry{goTool(), folder("build", "~/work/build")}

	all := results(newEngine(t, f), entries)
	if len(all) != 2 {
		t.Fatalf("got %d results, want one per place: %+v", len(all), all)
	}
	for _, result := range all {
		switch result.Place.Entry {
		case "gotool":
			if result.Place != Places(entries)[0] || len(result.Rows) != 0 {
				t.Fatalf("the absent place delivered %+v", result)
			}
		case "build":
			if len(result.Rows) != 1 {
				t.Fatalf("the folder delivered %+v", result)
			}
		default:
			t.Fatalf("a result for an unplanned place: %+v", result)
		}
	}
}

func TestVersionsPlaceWhoseListFailsCouldNotBeDetermined(t *testing.T) {
	for name, result := range map[string]fake.Result{
		"fails": fake.Fail(1, "corrupt settings file"),
		"hangs": fake.Hang(),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake()
			f.AddExecutable("chain")
			f.Script(argv("chain home"), fake.Out("/data/chain\n"))
			f.Script(argv("chain list"), result)
			f.AddDir("/data/chain/toolchains/stable", 1000)

			row := one(t, scan(t, f, chainTool()))
			want := "corrupt settings file"
			if name == "hangs" {
				want = "timed out after 40ms"
			}
			wantState(t, row, Undetermined, want)
			if row.Version != "" {
				t.Fatalf("the row names version %q", row.Version)
			}
		})
	}
}

func TestVersionsPlaceThatListsNothingGivesNoRow(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("\n"))
	f.AddDir("/data/chain/toolchains", 0)

	if rows := scan(t, f, chainTool()); len(rows) != 0 {
		t.Fatalf("got rows: %+v", rows)
	}
}

func TestVersionWhoseDirectoryWouldLeaveThePlaceIsNotMeasured(t *testing.T) {
	f := newFake()
	f.AddExecutable("chain")
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("..\n.\n"))
	f.AddDir("/data/chain/toolchains", 10)

	rows := scan(t, f, chainTool())
	if len(rows) != 2 {
		t.Fatalf("got %d rows: %+v", len(rows), rows)
	}
	wantState(t, rows[0], NotMeasured, "outside")
	wantState(t, rows[1], NotMeasured, "the whole place")
	for _, row := range rows {
		if row.Dir != "" {
			t.Fatalf("version %q got the directory %q", row.Version, row.Dir)
		}
	}
}

func TestCommandsAreSplitByQuotesAndPlaceholdersStayOneArgument(t *testing.T) {
	tool := entry.Entry{Kind: entry.Tool, Name: "sizer", Executable: "sizer", Places: []entry.Place{{
		Name:  "store",
		Holds: entry.Cache,
		Path:  &entry.Path{Env: "SIZER_STORE"},
		Measure: entry.Measure{
			By:      entry.Command,
			Command: `sizer du --format '{{.Type}} {{.Size}}' --dir <path> "--label=<path>/x"`,
			Pattern: `^total (\d+)$`,
		},
	}}}
	f := newFake()
	f.AddExecutable("sizer")
	f.SetEnv("SIZER_STORE", "/data/my store")
	f.AddDir("/data/my store", 1)
	f.Script(
		[]string{"sizer", "du", "--format", "{{.Type}} {{.Size}}", "--dir", "/data/my store", "--label=/data/my store/x"},
		fake.Out("total 4096\n"),
	)

	wantMeasured(t, one(t, scan(t, f, tool)), "/data/my store", 4096)
}

func TestMeasurePathNamesTheDirectoryInsideThePlace(t *testing.T) {
	tool := goTool()
	tool.Places[0].Measure.Path = "<path>/download"
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Fail(1, ""))
	f.AddDir(home+"/.cache/gotool", 300)
	f.AddDir(home+"/.cache/gotool/download", 40)

	wantMeasured(t, one(t, scan(t, f, tool)), home+"/.cache/gotool/download", 40)
}

func TestScanDeliversResultsAsTheyArrive(t *testing.T) {
	release := make(chan struct{})
	slow := goTool()
	slow.Name = "slow"
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Result{Stdout: "/data/slow\n", Effect: func() { <-release }})
	f.AddDir("/data/slow", 1)
	f.AddDir(home+"/work/build", 2)

	settings := testSettings()
	settings.CommandTimeout = time.Minute
	e, err := New(f, settings)
	if err != nil {
		t.Fatal(err)
	}
	results := e.Scan(context.Background(), []entry.Entry{slow, folder("build", "~/work/build")})

	select {
	case first := <-results:
		if first.Place.Entry != "build" || len(first.Rows) != 1 {
			t.Fatalf("the first result is %+v, want the place that was ready with its row", first)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no result arrived while another place was still being detected")
	}
	close(release)
	second, open := <-results
	if !open || second.Place.Entry != "slow" || len(second.Rows) != 1 {
		t.Fatalf("the second result is %+v (open: %v)", second, open)
	}
	if _, open := <-results; open {
		t.Fatal("the channel was not closed after the last result")
	}
}

func TestScanRunsNoMorePlacesAtOnceThanTheLimit(t *testing.T) {
	const places, limit = 12, 3
	var running, peak atomic.Int32
	f := newFake()
	f.AddExecutable("gotool")
	var entries []entry.Entry
	for i := range places {
		name := string(rune('a' + i))
		tool := goTool()
		tool.Name = name
		tool.Places[0].Path = &entry.Path{Command: "gotool where " + name}
		f.Script(argv("gotool where "+name), fake.Result{Stdout: "/data/" + name + "\n", Effect: func() {
			now := running.Add(1)
			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
		}})
		f.AddDir("/data/"+name, int64(i+1))
		entries = append(entries, tool)
	}
	settings := testSettings()
	settings.CommandTimeout = time.Minute
	settings.Parallel = limit
	e, err := New(f, settings)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, row := range flatten(t, entries, results(e, entries)) {
		if row.State != Measured {
			t.Errorf("%s: %s", row.Entry, row.Reason)
		}
		count++
	}
	if count != places {
		t.Fatalf("got %d rows, want %d", count, places)
	}
	if got := peak.Load(); got > limit {
		t.Fatalf("%d places ran at once, the limit is %d", got, limit)
	}
	if got := peak.Load(); got < 2 {
		t.Fatalf("at most %d place ran at once: the scan is not parallel", got)
	}
}

func TestScanStopsWhenTheContextEnds(t *testing.T) {
	f := newFake()
	f.AddExecutable("gotool")
	f.Script(argv("gotool env CACHE"), fake.Hang())
	settings := testSettings()
	settings.CommandTimeout = time.Hour
	e, err := New(f, settings)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows := e.Scan(ctx, []entry.Entry{goTool()})
	cancel()
	select {
	case <-rows:
	case <-time.After(10 * time.Second):
		t.Fatal("the scan did not end with its context")
	}
}

func TestUninstalledToolIsMeasuredByDiskUsageWithoutItsCommands(t *testing.T) {
	tool := goTool()
	tool.Places[0].Requires = &entry.Requires{Command: "gotool ready"}
	tool.Places[0].Measure = entry.Measure{
		By:      entry.Command,
		Command: "gotool size <path>",
		Pattern: `^(\d+)$`,
		Path:    "<path>/download",
	}
	f := newFake()
	f.AddDir(goCache, 300)
	f.AddDir(goCache+"/download", 40)
	f.Script(argv("gotool env CACHE"), fake.Out("/data/elsewhere\n"))
	f.Script(argv("gotool ready"), fake.Fail(1, "not ready"))
	f.Script(argv("gotool size "+goCache), fake.Out("7\n"))

	wantMeasured(t, one(t, scan(t, f, tool)), goCache, 340)
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands of an uninstalled tool were run: %v", commands)
	}
}

func TestUninstalledToolWithRequiresHasASize(t *testing.T) {
	tool := goTool()
	tool.Places[0].Requires = &entry.Requires{Command: "gotool ready"}
	f := newFake()
	f.AddDir(goCache, 300)
	f.Script(argv("gotool ready"), fake.Fail(1, "not ready"))

	wantMeasured(t, one(t, scan(t, f, tool)), goCache, 300)
	if slices.Contains(ran(f), "gotool ready") {
		t.Fatal("requires was run for an uninstalled tool")
	}
}

func TestUninstalledVersionsPlaceIsOneRowForTheWholeDirectory(t *testing.T) {
	const toolchains = home + "/.chain/toolchains"
	f := newFake()
	f.AddDir(toolchains+"/stable", 1000)
	f.AddDir(toolchains+"/nightly", 2000)
	f.Script(argv("chain home"), fake.Out("/data/chain\n"))
	f.Script(argv("chain list"), fake.Out("stable\nnightly\n"))

	row := one(t, scan(t, f, chainTool()))
	wantMeasured(t, row, toolchains, 3000)
	if row.Version != "" || row.Holds != entry.Versions {
		t.Fatalf("row has version %q and holds %q", row.Version, row.Holds)
	}
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands of an uninstalled tool were run: %v", commands)
	}
	if selected := Everything([]Row{row}); len(selected) != 0 {
		t.Fatalf("clean everything selected the row of a versions place: %+v", selected)
	}

	wantFreed(t, clean(t, f, row)[0], 3000)
	if !slices.Equal(f.Removed(), []string{toolchains}) {
		t.Fatalf("removed %v", f.Removed())
	}
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands of an uninstalled tool were run while cleaning: %v", commands)
	}
}

func TestUninstalledToolWhoseWalkFailsIsNotMeasured(t *testing.T) {
	tool := goTool()
	tool.Places[0].Measure = entry.Measure{By: entry.Command, Command: "gotool size", Pattern: `^(\d+)$`}
	f := newFake()
	f.AddDir(goCache, 300)
	f.HangDiskUsage(goCache)

	wantState(t, one(t, scan(t, f, tool)), NotMeasured, "timed out after 40ms")
	if commands := ran(f); len(commands) != 0 {
		t.Fatalf("commands were run: %v", commands)
	}
}
