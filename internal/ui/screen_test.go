package ui

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
)

func TestScanShowsRowsInStableOrder(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	if len(s.m.rows) != rowCount {
		t.Fatalf("got %d rows, want %d:\n%s", len(s.m.rows), rowCount, s.text())
	}
	var got []string
	for _, row := range s.m.rows {
		got = append(got, strings.TrimSpace(row.Entry+" "+row.Version))
	}
	want := []string{"alpha", "broken", "chain v1", "chain v2", "dark", "lost", "scratch"}
	if !slices.Equal(got, want) {
		t.Fatalf("got rows %q, want %q", got, want)
	}
}

func TestEverythingMarkNeverMarksAVersionsRow(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)

	s.press("a")
	want := []string{"alpha", "broken", "dark", "scratch"}
	if got := s.markedNames(); !slices.Equal(got, want) {
		t.Fatalf("after a: marked %q, want %q", got, want)
	}

	s.press("a")
	if got := s.markedNames(); len(got) != 0 {
		t.Fatalf("after a second a: marked %q, want nothing", got)
	}
}

func TestEverythingMarkLeavesAVersionMarkedByHandAlone(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)

	s.mark(rowChainV1)
	s.press("a")
	if got, want := s.markedNames(), []string{"alpha", "broken", "chain v1", "dark", "scratch"}; !slices.Equal(got, want) {
		t.Fatalf("after a: marked %q, want %q", got, want)
	}
	s.press("a")
	if got, want := s.markedNames(), []string{"chain v1"}; !slices.Equal(got, want) {
		t.Fatalf("after a second a: marked %q, want %q", got, want)
	}
}

func TestUndeterminedRowCannotBeMarked(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)

	s.mark(rowLost)
	if got := s.markedNames(); len(got) != 0 {
		t.Fatalf("marked %q, want nothing", got)
	}
	s.mark(rowDark)
	if got, want := s.markedNames(), []string{"dark"}; !slices.Equal(got, want) {
		t.Fatalf("a row that is not measured: marked %q, want %q", got, want)
	}
}

func TestEnterWithNothingMarkedDoesNothing(t *testing.T) {
	f, eng := fixture(t)
	s := scanned(t, eng)

	if cmd := s.send("enter"); cmd != nil {
		t.Fatal("enter with nothing marked started something")
	}
	if s.m.stage != stageList {
		t.Fatalf("stage %d, want the list", s.m.stage)
	}
	s.press("y")
	if ran := cleanCommandsRan(f); len(ran) != 0 || len(f.Removed()) != 0 {
		t.Fatalf("cleaned without a mark: ran %q, removed %q", ran, f.Removed())
	}
}

func TestConfirmationProceedsOnlyOnY(t *testing.T) {
	f, eng := fixture(t)
	s := scanned(t, eng)
	s.mark(rowAlpha, rowScratch)
	marked := s.markedNames()

	for _, key := range []string{"n", "enter", "space", "q", "ctrl+c", "Y"} {
		s.press("enter")
		if s.m.stage != stageConfirm {
			t.Fatalf("before %q: stage %d, want the confirmation", key, s.m.stage)
		}
		s.press(key)
		if s.m.stage != stageList || s.quit {
			t.Fatalf("after %q: stage %d, quit %v; want the list", key, s.m.stage, s.quit)
		}
		if got := s.markedNames(); !slices.Equal(got, marked) {
			t.Fatalf("after %q: marked %q, want %q kept", key, got, marked)
		}
		if ran := cleanCommandsRan(f); len(ran) != 0 || len(f.Removed()) != 0 {
			t.Fatalf("after %q: ran %q, removed %q; want nothing cleaned", key, ran, f.Removed())
		}
	}

	s.press("enter")
	text := s.text()
	for _, want := range []string{"alpha · cache", "1.5 kB", "scratch · /work/scratch", "700 B", "clean 2 rows · total 2.2 kB"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the confirmation lacks %q:\n%s", want, text)
		}
	}
	s.press("y")
	if s.m.stage != stageResult {
		t.Fatalf("after y: stage %d, want the result", s.m.stage)
	}
	if got, want := cleanCommandsRan(f), []string{"alpha clean"}; !slices.Equal(got, want) {
		t.Fatalf("ran %q, want %q", got, want)
	}
	if got, want := f.Removed(), []string{"/work/scratch"}; !slices.Equal(got, want) {
		t.Fatalf("removed %q, want %q", got, want)
	}
}

func TestFailedRowDoesNotStopTheOthers(t *testing.T) {
	f, eng := fixture(t)
	s := scanned(t, eng)
	s.mark(rowAlpha, rowBroken, rowScratch)
	s.press("enter", "y")

	if s.m.stage != stageResult {
		t.Fatalf("stage %d, want the result", s.m.stage)
	}
	if got, want := s.m.totals(), (Totals{Freed: 2200, Failed: 1}); got != want {
		t.Fatalf("totals %+v, want %+v", got, want)
	}
	if got, want := f.Removed(), []string{"/work/scratch"}; !slices.Equal(got, want) {
		t.Fatalf("removed %q, want %q: the row after the failed one was not cleaned", got, want)
	}
	text := s.text()
	for _, want := range []string{"✓ freed 1.5 kB", "✗ failed", "boom: the cache is locked", "✓ freed 700 B", "freed 2.2 kB · 1 failed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the result lacks %q:\n%s", want, text)
		}
	}

	s.press("x")
	if s.m.stage != stageList || s.m.scanning {
		t.Fatalf("after a key: stage %d, scanning %v; want the list after a new scan", s.m.stage, s.m.scanning)
	}
	if len(s.markedNames()) != 0 {
		t.Fatalf("marks survived the new scan: %q", s.markedNames())
	}
	for _, row := range s.m.rows {
		if row.Entry == "alpha" || row.Entry == "scratch" {
			t.Fatalf("the cleaned row %q is still listed after the new scan", row.Entry)
		}
	}
	if len(s.m.rows) != rowCount-2 {
		t.Fatalf("got %d rows after the new scan, want %d", len(s.m.rows), rowCount-2)
	}
}

func TestUnknownFreedSizeIsNotANumber(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.mark(rowDark, rowScratch)

	s.press("enter")
	if text := s.text(); !strings.Contains(text, "total 700 B · 1 not measured") {
		t.Fatalf("the confirmation does not say that a row has no size:\n%s", text)
	}
	s.press("y")

	if got, want := s.m.totals(), (Totals{Freed: 700, Unknown: 1}); got != want {
		t.Fatalf("totals %+v, want %+v", got, want)
	}
	text := s.text()
	for _, want := range []string{"✓ freed size unknown", "the size before cleaning is not known", "freed 700 B · 0 failed · 1 with freed size unknown"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the result lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "freed 0 B") {
		t.Fatalf("an unknown freed size is shown as zero:\n%s", text)
	}
}

func TestCtrlCStopsBeforeTheNextRow(t *testing.T) {
	f, eng := fixture(t)
	s := scanned(t, eng)
	s.mark(rowAlpha, rowScratch)
	s.press("enter")

	started, ok := s.send("y")().(tea.BatchMsg)
	if !ok || s.m.stage != stageCleaning {
		t.Fatalf("y did not start cleaning: stage %d", s.m.stage)
	}
	// The first row is in progress: its result exists but has not reached the screen.
	inProgress := started[0]()
	s.press("ctrl+c")
	if s.quit || s.m.stage != stageCleaning {
		t.Fatalf("ctrl+c during cleaning: quit %v, stage %d; want cleaning to go on", s.quit, s.m.stage)
	}
	_, next := s.m.Update(inProgress)
	s.run(next)

	if s.m.stage != stageResult {
		t.Fatalf("stage %d, want the result", s.m.stage)
	}
	if got, want := s.m.totals(), (Totals{Freed: 1500}); got != want {
		t.Fatalf("totals %+v, want %+v: the row in progress finishes", got, want)
	}
	if len(f.Removed()) != 0 {
		t.Fatalf("removed %q after ctrl+c, want the next row untouched", f.Removed())
	}
	if text := s.text(); !strings.Contains(text, "1 not cleaned") {
		t.Fatalf("the result does not say that a row was not cleaned:\n%s", text)
	}
}

func TestListShowsStatesProblemsAndTotals(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng, Problem{Source: "/etc/registry/odd.toml", Reason: "unknown key: colour"})
	s.m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	s.mark(rowAlpha, rowChainV2)

	text := s.text()
	for _, want := range []string{
		"── alpha ",
		"toolchains v2",
		"/work/scratch",
		"… not measured (",
		"✗ could not be determined (path.command: `lost where`: exit status 3: no configuration",
		"✗ /etc/registry/odd.toml: unknown key: colour",
		"marked 2 · 1.9 kB",
		"all caches 4.2 kB",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("the list lacks %q:\n%s", want, text)
		}
	}
}

func TestEmptyScanSaysNothingWasFound(t *testing.T) {
	_, eng := fixture(t)
	eng.entries = nil
	s := scanned(t, eng)
	if text := s.text(); !strings.Contains(text, "nothing found") {
		t.Fatalf("the empty list does not say that nothing was found:\n%s", text)
	}
	s.press("j", "G", "space", "a", "enter")
	if s.m.stage != stageList {
		t.Fatalf("stage %d, want the list", s.m.stage)
	}
}

func TestQuit(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		_, eng := fixture(t)
		s := scanned(t, eng)
		s.press(key)
		if !s.quit {
			t.Fatalf("%q did not quit", key)
		}
	}
}

func TestPanelFitsALowWindow(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	s.press("a")
	s.mark(rowChainV1, rowChainV2)
	s.press("enter")

	text := s.text()
	if lines := strings.Count(text, "\n") + 1; lines > 10 {
		t.Fatalf("the panel takes %d lines of a window of 10:\n%s", lines, text)
	}
	for _, want := range []string{"↓ ", " more", "total "} {
		if !strings.Contains(text, want) {
			t.Fatalf("the panel lacks %q:\n%s", want, text)
		}
	}
}

// The status band and the help line are the last lines of the window, however short the
// content above them is.
func TestFooterStaysAtTheBottomOfTheWindow(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	s.mark(rowAlpha)

	for _, screen := range []struct{ name, band, help string }{
		{"the list", "marked 1 · 1.5 kB", "q quit"},
		{"the confirmation", "clean 1 row · total 1.5 kB", "any other key back"},
	} {
		lines := strings.Split(s.text(), "\n")
		if len(lines) != 40 {
			t.Fatalf("%s takes %d lines of a window of 40:\n%s", screen.name, len(lines), s.text())
		}
		if band := lines[38]; !strings.Contains(band, screen.band) || lipgloss.Width(band) != 100 {
			t.Fatalf("%s: line 39 is %q, want the status band across 100 columns with %q", screen.name, band, screen.band)
		}
		if help := lines[39]; !strings.Contains(help, screen.help) {
			t.Fatalf("%s: the last line is %q, want the help with %q", screen.name, help, screen.help)
		}
		s.press("enter")
	}
}

func TestQuestionMarkTogglesTheFullHelp(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	short := s.text()
	if !strings.Contains(short, "? more") || strings.Contains(short, "down") {
		t.Fatalf("the list does not start with the short help:\n%s", short)
	}

	s.press("?")
	full := s.text()
	for _, want := range []string{"↑/k", "↓/j", "first", "last", "mark", "all caches", "clean", "? ", "less", "quit"} {
		if !strings.Contains(full, want) {
			t.Fatalf("the full help lacks %q:\n%s", want, full)
		}
	}
	if lines := strings.Count(full, "\n") + 1; lines != 40 {
		t.Fatalf("with the full help the list takes %d lines of a window of 40:\n%s", lines, full)
	}

	s.press("?")
	if got := s.text(); got != short {
		t.Fatalf("a second ? does not bring the short help back:\n%s", got)
	}
}

func TestListSaysHowManyRowsAreOutOfView(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})

	lines := strings.Split(s.text(), "\n")
	if len(lines) > 12 {
		t.Fatalf("the list takes %d lines of a window of 12:\n%s", len(lines), s.text())
	}
	below := -1
	for i, l := range lines {
		if strings.Contains(l, "↓ ") && strings.Contains(l, " more") {
			below = i
		}
	}
	if below < 1 {
		t.Fatalf("the list does not say that rows are below:\n%s", s.text())
	}
	if last := strings.TrimSpace(lines[below-1]); strings.HasPrefix(last, "──") {
		t.Fatalf("a group header is shown without its rows:\n%s", s.text())
	}
	if strings.Contains(s.text(), "↑ ") && strings.Contains(lines[1], " more") {
		t.Fatalf("rows are reported above the first row:\n%s", s.text())
	}

	s.press("G")
	text := s.text()
	after := strings.Split(text, "\n")
	if !strings.Contains(after[1], " more") {
		t.Fatalf("the list does not say that rows are above:\n%s", text)
	}
	if first := strings.TrimSpace(after[2]); first != "" && !strings.HasPrefix(first, "──") {
		t.Fatalf("rows are shown at the top without the name of their group:\n%s", text)
	}
	if strings.Contains(text, "↓ ") && strings.Contains(text, "more\n─") {
		t.Fatalf("rows are reported below the last row:\n%s", text)
	}
}

// stubEngine hands the screen the places and the results a test chooses.
type stubEngine struct {
	places []engine.Place
}

func (s stubEngine) Places() []engine.Place { return s.places }

func (s stubEngine) Scan(context.Context) <-chan engine.PlaceResult {
	return make(chan engine.PlaceResult)
}

func (s stubEngine) Clean(context.Context, []engine.Row) <-chan engine.Result {
	results := make(chan engine.Result)
	close(results)
	return results
}

func (s stubEngine) Everything(rows []engine.Row) []engine.Row { return engine.Everything(rows) }

// A tool with two places, a tool whose place turns out absent, and a folder.
var (
	stubCache   = engine.Place{Entry: "alpha", Name: "cache", EntryIndex: 0, PlaceIndex: 0}
	stubChains  = engine.Place{Entry: "alpha", Name: "toolchains", EntryIndex: 0, PlaceIndex: 1}
	stubGhost   = engine.Place{Entry: "ghost", Name: "cache", EntryIndex: 1, PlaceIndex: 0}
	stubScratch = engine.Place{Entry: "scratch", Kind: entry.Folder, Path: "/work/scratch", EntryIndex: 2, PlaceIndex: 0}
)

func rowOf(place engine.Place, row engine.Row) engine.Row {
	row.Entry, row.Place = place.Entry, place.Name
	row.EntryIndex, row.PlaceIndex = place.EntryIndex, place.PlaceIndex
	return row
}

// pendingScreen is a screen whose scan has started and delivered nothing yet.
func pendingScreen(t *testing.T) *screen {
	t.Helper()
	s := &screen{t: t, m: NewModel(stubEngine{places: []engine.Place{stubScratch, stubGhost, stubChains, stubCache}}, nil)}
	s.m.Init()
	return s
}

func (s *screen) deliver(place engine.Place, rows ...engine.Row) {
	for i := range rows {
		rows[i] = rowOf(place, rows[i])
	}
	s.m.Update(placeMsg{scan: s.m.scan, result: engine.PlaceResult{Place: place, Rows: rows}})
}

func TestEveryPlannedPlaceIsShownAsBeingMeasured(t *testing.T) {
	s := pendingScreen(t)

	if len(s.m.rows) != 4 || s.m.pending() != 4 {
		t.Fatalf("got %d rows, %d pending; want one pending row per planned place", len(s.m.rows), s.m.pending())
	}
	text := s.text()
	for _, want := range []string{"── alpha ", "· cache", "· toolchains", "── ghost ", "── scratch ", "· /work/scratch", "measuring 4"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the list lacks %q:\n%s", want, text)
		}
	}
	// One per pending row and one in the title.
	if got := strings.Count(text, "measuring"); got != 5 {
		t.Fatalf("\"measuring\" appears %d times, want 5:\n%s", got, text)
	}
	if strings.Contains(text, "nothing found") {
		t.Fatalf("a scan that has not finished says that nothing was found:\n%s", text)
	}
}

func TestPendingRowCannotBeMarked(t *testing.T) {
	s := pendingScreen(t)
	s.deliver(stubCache, engine.Row{Holds: entry.Cache, State: engine.Measured, Size: 1500})

	for i := range s.m.rows {
		s.mark(i)
	}
	if got, want := s.markedNames(), []string{"alpha"}; !slices.Equal(got, want) {
		t.Fatalf("space on every row: marked %q, want only the measured row %q", got, want)
	}
	// The first a unmarks the one covered row that is marked, the second marks again.
	s.press("space", "a")
	if got := s.markedNames(); len(got) != 0 {
		t.Fatalf("after a: marked %q, want nothing", got)
	}
	s.press("a")
	if got := s.markedNames(); !slices.Equal(got, []string{"alpha"}) {
		t.Fatalf("after a second a: marked %q, want only the measured row", got)
	}
	if len(s.m.marked) != 1 {
		t.Fatalf("%d marks are held for one markable row", len(s.m.marked))
	}
	if cmd := s.send("enter"); cmd != nil || len(s.m.items) != 1 {
		t.Fatalf("the confirmation holds %d rows, want the one measured row", len(s.m.items))
	}
}

func TestResultReplacesThePendingRow(t *testing.T) {
	s := pendingScreen(t)

	s.deliver(stubChains,
		engine.Row{Holds: entry.Versions, Version: "v1", VersionIndex: 0, State: engine.Measured, Size: 300},
		engine.Row{Holds: entry.Versions, Version: "v2", VersionIndex: 1, State: engine.Measured, Size: 400})
	var got []string
	for _, row := range s.m.rows {
		got = append(got, fmt.Sprintf("%s/%s/%s pending=%v", row.Entry, row.Place, row.Version, row.pending))
	}
	want := []string{
		"alpha/cache/ pending=true",
		"alpha/toolchains/v1 pending=false",
		"alpha/toolchains/v2 pending=false",
		"ghost/cache/ pending=true",
		"scratch// pending=true",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got rows %q, want %q", got, want)
	}
	text := s.text()
	for _, want := range []string{"toolchains v1", "toolchains v2", "measuring 3", "· cache"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the list lacks %q:\n%s", want, text)
		}
	}
}

func TestResultWithoutRowsDropsTheRowAndAnEmptiedGroup(t *testing.T) {
	s := pendingScreen(t)
	s.press("j", "j")
	if s.m.rows[s.m.cursor].Entry != "ghost" {
		t.Fatalf("the cursor is on %q, want the pending row of ghost", s.m.rows[s.m.cursor].Entry)
	}

	s.deliver(stubGhost)
	if text := s.text(); strings.Contains(text, "ghost") {
		t.Fatalf("an entry whose only place is absent is still shown:\n%s", text)
	}
	if len(s.m.rows) != 3 || s.m.rows[s.m.cursor].Entry != "scratch" {
		t.Fatalf("%d rows, cursor on %q; want 3 rows and the cursor on the next row", len(s.m.rows), s.m.rows[s.m.cursor].Entry)
	}

	// One of two places is absent: the group stays for the other.
	s.deliver(stubCache)
	text := s.text()
	if !strings.Contains(text, "── alpha ") || !strings.Contains(text, "· toolchains") || strings.Contains(text, "· cache") {
		t.Fatalf("the group of a tool with one absent place is wrong:\n%s", text)
	}

	s.deliver(stubChains)
	s.deliver(stubScratch)
	s.m.Update(scanDoneMsg{scan: s.m.scan})
	if text := s.text(); !strings.Contains(text, "nothing found") || !strings.Contains(text, "✓ done") {
		t.Fatalf("a scan that found nothing does not say so:\n%s", text)
	}
}

func TestCursorStaysOnItsRowWhenRowsArriveAbove(t *testing.T) {
	s := pendingScreen(t)
	s.press("G")
	s.deliver(stubChains,
		engine.Row{Holds: entry.Versions, Version: "v1", VersionIndex: 0, State: engine.Measured, Size: 300},
		engine.Row{Holds: entry.Versions, Version: "v2", VersionIndex: 1, State: engine.Measured, Size: 400})
	if row := s.m.rows[s.m.cursor]; row.Entry != "scratch" {
		t.Fatalf("the cursor moved to %q", row.Entry)
	}
}

func TestNewScanAfterCleaningStartsWithPendingRows(t *testing.T) {
	_, eng := fixture(t)
	s := scanned(t, eng)
	s.mark(rowAlpha)
	s.press("enter", "y")

	s.send("x")
	if s.m.stage != stageList || s.m.pending() != len(eng.Places()) || len(s.m.rows) != len(eng.Places()) {
		t.Fatalf("stage %d, %d rows, %d pending; want the list with one pending row per place",
			s.m.stage, len(s.m.rows), s.m.pending())
	}
}

func TestFolderThatCannotBeLookedAtIsLabelledWithItsPath(t *testing.T) {
	f, eng := fixture(t)
	f.SetStatError("/work/scratch", &fs.PathError{Op: "lstat", Path: "/work/scratch", Err: fs.ErrPermission})
	s := scanned(t, eng)
	s.m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	if text := s.text(); !strings.Contains(text, "○ /work/scratch  … not measured (lstat /work/scratch: permission denied)") {
		t.Fatalf("the folder row is not labelled with its path:\n%s", text)
	}
	var out bytes.Buffer
	WriteRows(&out, Collect(context.Background(), eng))
	if !strings.Contains(out.String(), "scratch  /work/scratch      not measured (") {
		t.Fatalf("the listing does not name the folder's path:\n%s", out.String())
	}
}
