package ui

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

// addMachine is a machine with the tool "chain": a cache of 1500 bytes that `chain
// cache-dir` prints, and two installed versions of 300 and 400 bytes.
func addMachine(t *testing.T) (*fake.System, *engine.Engine) {
	t.Helper()
	f := fake.New(sys.Linux, home)
	f.AddExecutable("chain")
	f.Script([]string{"chain", "cache-dir"}, fake.Out(home+"/.cache/chain\n"))
	f.AddDir(home+"/.cache/chain", 1500)
	f.Script([]string{"chain", "home"}, fake.Out(home+"/.chain\n"))
	f.Script([]string{"chain", "list"}, fake.Out("v1 (default)\nv2\n"))
	f.AddDir(home+"/.chain/v1", 300)
	f.AddDir(home+"/.chain/v2", 400)
	f.Script([]string{"chain", "size"}, fake.Out("Total 4.2MB\n"))
	f.Script([]string{"chain", "clean"}, fake.Result{Effect: func() { f.Delete(home + "/.cache/chain") }})
	f.Script([]string{"chain", "uninstall", "v1"}, fake.Result{Effect: func() { f.Delete(home + "/.chain/v1") }})
	eng, err := engine.New(f, engine.Settings{
		CommandTimeout: time.Second,
		WalkTimeout:    time.Second,
		CleanTimeout:   time.Second,
		Parallel:       4,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f, eng
}

func chainAnswers() ToolAnswers {
	return ToolAnswers{Name: "chain", Executable: "chain", OS: sys.Linux}
}

func cacheAnswers() PlaceAnswers {
	return PlaceAnswers{
		Name:         "cache",
		Holds:        entry.Cache,
		PathCommand:  "chain cache-dir",
		PathEnv:      "CHAIN_CACHE",
		PathDefault:  "~/.cache/chain",
		MeasureBy:    entry.DiskUsage,
		CleanCommand: "chain clean",
	}
}

func versionAnswers() PlaceAnswers {
	return PlaceAnswers{
		Name:         "versions",
		Holds:        entry.Versions,
		PathCommand:  "chain home",
		ListCommand:  "chain list",
		ListPattern:  `^(\S+)`,
		VersionDir:   "<path>/<version>",
		MeasureBy:    entry.DiskUsage,
		CleanCommand: "chain uninstall <version>",
	}
}

func TestAnswersGiveTheEntryOfACachePlace(t *testing.T) {
	a := chainAnswers()
	a.Places = []PlaceAnswers{cacheAnswers()}
	want := entry.Entry{Kind: entry.Tool, Name: "chain", Executable: "chain", Places: []entry.Place{{
		Name:    "cache",
		Holds:   entry.Cache,
		Path:    &entry.Path{Command: "chain cache-dir", Env: "CHAIN_CACHE", Default: map[string]string{sys.Linux: "~/.cache/chain"}},
		Measure: entry.Measure{By: entry.DiskUsage},
		Clean:   entry.Clean{Command: "chain clean"},
	}}}
	if got := a.Entry(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the entry is\n%#v\nwant\n%#v", got, want)
	}
	if err := entry.Validate(a.Entry()); err != nil {
		t.Fatal(err)
	}
}

func TestAnswersGiveTheEntryOfAVersionsPlace(t *testing.T) {
	a := chainAnswers()
	a.Places = []PlaceAnswers{versionAnswers()}
	want := entry.Entry{Kind: entry.Tool, Name: "chain", Executable: "chain", Places: []entry.Place{{
		Name:    "versions",
		Holds:   entry.Versions,
		Path:    &entry.Path{Command: "chain home"},
		List:    &entry.List{Command: "chain list", Pattern: `^(\S+)`},
		Measure: entry.Measure{By: entry.DiskUsage, Path: "<path>/<version>"},
		Clean:   entry.Clean{Command: "chain uninstall <version>"},
	}}}
	if got := a.Entry(); !reflect.DeepEqual(got, want) {
		t.Fatalf("the entry is\n%#v\nwant\n%#v", got, want)
	}
	if err := entry.Validate(a.Entry()); err != nil {
		t.Fatal(err)
	}
}

func TestMeasuringByACommandAndCleaningByRemovalReachTheEntry(t *testing.T) {
	p := cacheAnswers()
	p.MeasureBy, p.MeasureCommand, p.MeasurePattern = entry.Command, "chain size", `^Total (\S+)`
	p.CleanCommand = ""
	a := chainAnswers()
	a.Places = []PlaceAnswers{p}
	got := a.Entry().Places[0]
	if want := (entry.Measure{By: entry.Command, Command: "chain size", Pattern: `^Total (\S+)`}); got.Measure != want {
		t.Fatalf("measure is %#v, want %#v", got.Measure, want)
	}
	if got.Clean != (entry.Clean{}) {
		t.Fatalf("clean is %#v, want removal of the directory", got.Clean)
	}
}

func TestAStepNamesTheRulesItsAnswersBreak(t *testing.T) {
	a := chainAnswers()

	noSource := cacheAnswers()
	noSource.PathCommand, noSource.PathEnv, noSource.PathDefault = "", "", ""
	if got := a.Problems(noSource, throughDetection); len(got) != 1 || !strings.Contains(got[0], "path gives no source") {
		t.Fatalf("a place without a source of its path: %q", got)
	}

	wholePlace := versionAnswers()
	wholePlace.VersionDir = "<path>"
	if got := a.Problems(wholePlace, throughDetection); len(got) != 1 || !strings.Contains(got[0], "measure.path must contain <version>") {
		t.Fatalf("a version directory that is the whole place: %q", got)
	}

	// The space check is not asked about yet, so its answers are not judged.
	early := cacheAnswers()
	early.MeasureBy = entry.Command
	if got := a.Problems(early, throughDetection); got != nil {
		t.Fatalf("at detection: %q", got)
	}
	if got := a.Problems(early, throughSpace); len(got) != 2 {
		t.Fatalf("a command measure without command and pattern: %q", got)
	}
	if got := a.Problems(cacheAnswers(), throughClean); got != nil {
		t.Fatalf("complete answers: %q", got)
	}
}

func TestANameOfTheUserIsRefused(t *testing.T) {
	has := func(name string) (bool, error) { return name == "mine", nil }
	if err := NameProblem("mine", has); err == nil || !strings.Contains(err.Error(), `already have an entry named "mine"`) {
		t.Fatalf("a name the user has: %v", err)
	}
	if err := NameProblem("fresh", has); err != nil {
		t.Fatalf("a new name: %v", err)
	}
	if err := NameProblem("", has); err == nil {
		t.Fatal("an empty name was accepted")
	}
	broken := func(string) (bool, error) { return false, errors.New("the registry cannot be read") }
	if err := NameProblem("fresh", broken); err == nil {
		t.Fatal("a name that could not be looked up was accepted")
	}
}

func TestAPlaceNameIsUniqueInsideTheEntry(t *testing.T) {
	a := chainAnswers()
	a.Places = []PlaceAnswers{cacheAnswers()}
	if err := a.PlaceNameProblem("cache"); err == nil {
		t.Fatal("the name of another place was accepted")
	}
	if err := a.PlaceNameProblem("versions"); err != nil {
		t.Fatal(err)
	}
}

func TestDetectionCheckShowsThePathFound(t *testing.T) {
	_, eng := addMachine(t)
	check := CheckDetection(context.Background(), eng, chainAnswers(), cacheAnswers())
	if check.Failure != "" || check.Path != home+"/.cache/chain" {
		t.Fatalf("the check gave %+v", check)
	}
}

func TestDetectionCheckListsTheVersions(t *testing.T) {
	_, eng := addMachine(t)
	check := CheckDetection(context.Background(), eng, chainAnswers(), versionAnswers())
	if check.Failure != "" || check.Path != home+"/.chain" {
		t.Fatalf("the check gave %+v", check)
	}
	if got := check.Versions(); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Fatalf("the versions are %q", got)
	}
}

func TestDetectionCheckReportsWhyItFailed(t *testing.T) {
	f, eng := addMachine(t)
	f.Script([]string{"chain", "cache-dir"}, fake.Fail(3, "no configuration"))
	p := cacheAnswers()
	p.PathEnv, p.PathDefault = "", ""
	check := CheckDetection(context.Background(), eng, chainAnswers(), p)
	if !strings.Contains(check.Failure, "no configuration") || check.Path != "" {
		t.Fatalf("a path command that fails: %+v", check)
	}

	absent := cacheAnswers()
	absent.PathCommand, absent.PathEnv, absent.PathDefault = "", "", "~/.cache/nowhere"
	if check := CheckDetection(context.Background(), eng, chainAnswers(), absent); !strings.Contains(check.Failure, "was not found on this machine") {
		t.Fatalf("a directory that does not exist: %+v", check)
	}

	f.Script([]string{"chain", "list"}, fake.Fail(1, "the index is broken"))
	if check := CheckDetection(context.Background(), eng, chainAnswers(), versionAnswers()); !strings.Contains(check.Failure, "the index is broken") {
		t.Fatalf("a list command that fails: %+v", check)
	}
}

func TestDetectionCheckSaysWhenTheExecutableIsNotOnPath(t *testing.T) {
	f, eng := addMachine(t)
	f.RemoveExecutable("chain")
	check := CheckDetection(context.Background(), eng, chainAnswers(), cacheAnswers())
	if !strings.Contains(check.Failure, "chain is not on PATH") || check.Path != home+"/.cache/chain" {
		t.Fatalf("the check gave %+v", check)
	}
	if ran := f.Ran(); len(ran) != 0 {
		t.Fatalf("commands ran: %q", ran)
	}
}

func TestSpaceCheckShowsTheSize(t *testing.T) {
	_, eng := addMachine(t)
	check := CheckSpace(context.Background(), eng, chainAnswers(), cacheAnswers())
	if want := []CheckRow{{Measured: true, Size: 1500}}; check.Failure != "" || !reflect.DeepEqual(check.Rows, want) {
		t.Fatalf("disk usage: %+v", check)
	}

	p := cacheAnswers()
	p.MeasureBy, p.MeasureCommand, p.MeasurePattern = entry.Command, "chain size", `^Total (\S+)`
	check = CheckSpace(context.Background(), eng, chainAnswers(), p)
	if want := []CheckRow{{Measured: true, Size: 4_200_000}}; check.Failure != "" || !reflect.DeepEqual(check.Rows, want) {
		t.Fatalf("a command: %+v", check)
	}

	check = CheckSpace(context.Background(), eng, chainAnswers(), versionAnswers())
	want := []CheckRow{{Version: "v1", Measured: true, Size: 300}, {Version: "v2", Measured: true, Size: 400}}
	if check.Failure != "" || !reflect.DeepEqual(check.Rows, want) {
		t.Fatalf("versions: %+v", check)
	}
}

func TestSpaceCheckReportsWhyARowHasNoSize(t *testing.T) {
	_, eng := addMachine(t)
	p := cacheAnswers()
	p.MeasureBy, p.MeasureCommand, p.MeasurePattern = entry.Command, "chain size", `^Reclaimable (\S+)`
	check := CheckSpace(context.Background(), eng, chainAnswers(), p)
	if !strings.Contains(check.Failure, "no line of the output matches measure.pattern") {
		t.Fatalf("a pattern that matches nothing: %+v", check)
	}

	missing := versionAnswers()
	missing.VersionDir = "<path>/toolchains/<version>"
	check = CheckSpace(context.Background(), eng, chainAnswers(), missing)
	if !strings.HasPrefix(check.Failure, "v1 not measured") {
		t.Fatalf("a version directory that does not exist: %+v", check)
	}
}

func TestTheChecksNeverRunACleanCommand(t *testing.T) {
	f, eng := addMachine(t)
	for _, p := range []PlaceAnswers{cacheAnswers(), versionAnswers()} {
		CheckDetection(context.Background(), eng, chainAnswers(), p)
		CheckSpace(context.Background(), eng, chainAnswers(), p)
	}
	for _, argv := range f.Ran() {
		if line := strings.Join(argv, " "); !slices.Contains([]string{"chain cache-dir", "chain home", "chain list"}, line) {
			t.Fatalf("the checks ran %q", line)
		}
	}
	if removed := f.Removed(); len(removed) != 0 {
		t.Fatalf("the checks removed %q", removed)
	}
}
