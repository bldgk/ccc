package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

const home = "/home/alice"

// bound is the engine together with the entries of a test.
type bound struct {
	engine  *engine.Engine
	entries []entry.Entry
}

func (b bound) Places() []engine.Place { return engine.Places(b.entries) }

func (b bound) Scan(ctx context.Context) <-chan engine.PlaceResult {
	return b.engine.Scan(ctx, b.entries)
}

func (b bound) Clean(ctx context.Context, rows []engine.Row) <-chan engine.Result {
	return b.engine.Clean(ctx, rows)
}

func (b bound) Everything(rows []engine.Row) []engine.Row { return engine.Everything(rows) }

func cacheTool(name string) entry.Entry {
	return entry.Entry{Kind: entry.Tool, Name: name, Executable: name, Places: []entry.Place{{
		Name:    "cache",
		Holds:   entry.Cache,
		Path:    &entry.Path{Default: map[string]string{sys.Linux: "~/.cache/" + name}},
		Measure: entry.Measure{By: entry.DiskUsage},
		Clean:   entry.Clean{Command: name + " clean"},
	}}}
}

// The rows of the fixture, in the order of the screen.
const (
	rowAlpha = iota
	rowBroken
	rowChainV1
	rowChainV2
	rowDark
	rowLost
	rowScratch
	rowCount
)

// fixture is a machine with: a cache that cleans (1500 bytes), a cache whose clean
// command fails (2000), two installed versions (300 and 400), a cache that cannot be
// measured, a place that cannot be determined, and a folder (700).
func fixture(t *testing.T) (*fake.System, bound) {
	t.Helper()
	f := fake.New(sys.Linux, home)

	alpha := cacheTool("alpha")
	f.AddExecutable("alpha")
	f.AddDir(home+"/.cache/alpha", 1500)
	f.Script([]string{"alpha", "clean"}, fake.Result{Effect: func() { f.Delete(home + "/.cache/alpha") }})

	broken := cacheTool("broken")
	f.AddExecutable("broken")
	f.AddDir(home+"/.cache/broken", 2000)
	f.Script([]string{"broken", "clean"}, fake.Fail(1, "boom:\n the cache is locked"))

	chain := entry.Entry{Kind: entry.Tool, Name: "chain", Executable: "chain", Places: []entry.Place{{
		Name:    "toolchains",
		Holds:   entry.Versions,
		Path:    &entry.Path{Command: "chain home", Subdir: "toolchains", Default: map[string]string{sys.Linux: "~/.chain/toolchains"}},
		List:    &entry.List{Command: "chain list", Pattern: `^(\S+)`},
		Measure: entry.Measure{By: entry.DiskUsage, Path: "<path>/<version>"},
		Clean:   entry.Clean{Command: "chain uninstall <version>"},
	}}}
	f.AddExecutable("chain")
	f.Script([]string{"chain", "home"}, fake.Out(home+"/.chain\n"))
	f.Script([]string{"chain", "list"}, fake.Out("v1\nv2\n"))
	f.AddDir(home+"/.chain/toolchains/v1", 300)
	f.AddDir(home+"/.chain/toolchains/v2", 400)
	f.Script([]string{"chain", "uninstall", "v1"}, fake.Result{Effect: func() { f.Delete(home + "/.chain/toolchains/v1") }})

	dark := cacheTool("dark")
	dark.Places[0].Clean = entry.Clean{}
	f.AddExecutable("dark")
	f.AddDir(home+"/.cache/dark", 10)
	f.AddDir(home+"/.cache/dark/locked", 10)
	f.SetUnreadable(home + "/.cache/dark/locked")

	lost := cacheTool("lost")
	lost.Places[0].Path = &entry.Path{Command: "lost where"}
	f.AddExecutable("lost")
	f.Script([]string{"lost", "where"}, fake.Fail(3, "no configuration"))

	scratch := entry.Entry{Kind: entry.Folder, Name: "scratch", Path: "/work/scratch"}
	f.AddDir("/work/scratch", 700)

	entries := []entry.Entry{alpha, broken, chain, dark, lost, scratch}
	for _, e := range entries {
		if err := entry.Validate(e); err != nil {
			t.Fatalf("the test's entry %q is not valid: %v", e.Name, err)
		}
	}
	eng, err := engine.New(f, engine.Settings{
		CommandTimeout: time.Second,
		WalkTimeout:    time.Second,
		CleanTimeout:   time.Second,
		Parallel:       4,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f, bound{engine: eng, entries: entries}
}

// screen drives the model the way the terminal program does, without a terminal.
type screen struct {
	t    *testing.T
	m    *Model
	quit bool
}

// scanned returns a screen whose first scan is over.
func scanned(t *testing.T, eng Engine, problems ...Problem) *screen {
	t.Helper()
	s := &screen{t: t, m: NewModel(eng, problems)}
	s.run(s.m.Init())
	if s.m.scanning {
		t.Fatal("the scan did not finish")
	}
	return s
}

// run executes a command and everything that follows from it. Spinner ticks are
// dropped: nothing in the tests depends on the animation.
func (s *screen) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			s.run(c)
		}
	case spinner.TickMsg:
	case tea.QuitMsg:
		s.quit = true
	default:
		_, next := s.m.Update(msg)
		s.run(next)
	}
}

func keyMsg(key string) tea.KeyPressMsg {
	switch key {
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(key)[0]
	return tea.KeyPressMsg{Code: r, Text: key}
}

// send delivers one key and returns the command it produced, not yet run.
func (s *screen) send(key string) tea.Cmd {
	s.t.Helper()
	msg := keyMsg(key)
	if msg.String() != key {
		s.t.Fatalf("the key message for %q reads %q", key, msg.String())
	}
	_, cmd := s.m.Update(msg)
	return cmd
}

// press delivers keys and lets everything they start run to its end.
func (s *screen) press(keys ...string) {
	s.t.Helper()
	for _, key := range keys {
		s.run(s.send(key))
	}
}

// mark puts the cursor on the rows given and presses space on each.
func (s *screen) mark(rows ...int) {
	s.t.Helper()
	for _, row := range rows {
		s.press("g")
		for range row {
			s.press("j")
		}
		s.press("space")
	}
}

func (s *screen) markedNames() []string {
	var names []string
	for _, row := range s.m.markedRows() {
		names = append(names, strings.TrimSpace(row.Entry+" "+row.Version))
	}
	return names
}

func (s *screen) text() string {
	return ansi.Strip(s.m.render())
}

func cleanCommandsRan(f *fake.System) []string {
	var commands []string
	for _, c := range f.Ran() {
		line := strings.Join(c, " ")
		if strings.HasSuffix(line, " clean") || strings.Contains(line, " uninstall ") {
			commands = append(commands, line)
		}
	}
	return commands
}
