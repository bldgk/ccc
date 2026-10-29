package ui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
	"github.com/bldgk/ccc/internal/sys/fake"
)

// formRun drives the form the way the terminal program does, without a terminal:
// commands run on their own goroutines and their messages come back to the test's.
type formRun struct {
	t     *testing.T
	m     *AddModel
	fake  *fake.System
	msgs  chan tea.Msg
	quit  bool
	saved []entry.Entry
	taken []string
}

const shippedName = "shipped-tool"

func newFormRun(t *testing.T) *formRun {
	t.Helper()
	f, eng := addMachine(t)
	r := &formRun{t: t, fake: f, msgs: make(chan tea.Msg, 1024)}
	r.m = NewAddModel(r.deps(eng))
	r.exec(r.m.Init())
	r.until("the first question", func() bool { return r.focus() == keyName })
	return r
}

func (r *formRun) deps(eng *engine.Engine) AddForm {
	return AddForm{
		Scanner: eng,
		OS:      sys.Linux,
		UserHas: func(name string) (bool, error) { return slices.Contains(r.taken, name), nil },
		Shipped: func(name string) bool { return name == shippedName },
		Text: func(e entry.Entry) (string, error) {
			return fmt.Sprintf("name = %q\nplaces = %d\n", e.Name, len(e.Places)), nil
		},
		Save: func(e entry.Entry) (string, error) {
			r.saved = append(r.saved, e)
			return "/registry/" + e.Name + ".toml", nil
		},
	}
}

func (r *formRun) exec(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() { r.msgs <- cmd() }()
}

func (r *formRun) handle(msg tea.Msg) {
	switch msg := msg.(type) {
	case nil, spinner.TickMsg, cursor.BlinkMsg:
	case tea.QuitMsg:
		r.quit = true
	case tea.BatchMsg:
		for _, cmd := range msg {
			r.exec(cmd)
		}
	default:
		// A sequence is a slice of commands of a type the library keeps to itself.
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
			cmds := v.Convert(reflect.TypeFor[[]tea.Cmd]()).Interface().([]tea.Cmd)
			go func() {
				for _, cmd := range cmds {
					if cmd != nil {
						r.msgs <- cmd()
					}
				}
			}()
			return
		}
		_, cmd := r.m.Update(msg)
		r.exec(cmd)
	}
}

// until lets the form work until the condition holds.
func (r *formRun) until(what string, ok func() bool) {
	r.t.Helper()
	deadline := time.After(10 * time.Second)
	for !ok() {
		select {
		case msg := <-r.msgs:
			r.handle(msg)
		case <-deadline:
			r.t.Fatalf("the form did not reach %s; the screen:\n%s", what, r.text())
		}
	}
}

func (r *formRun) focus() string {
	if r.m.form == nil || r.m.phase == phaseChecking {
		return ""
	}
	return r.m.form.GetFocusedField().GetKey()
}

func (r *formRun) key(key string) {
	r.t.Helper()
	msg := keyMsg(key)
	if key == "esc" {
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	if msg.String() != key {
		r.t.Fatalf("the key message for %q reads %q", key, msg.String())
	}
	_, cmd := r.m.Update(msg)
	r.exec(cmd)
}

// answer types text into the question that has the focus and confirms it; the form then
// has to move the focus to next, or leave the questions when next is empty.
func (r *formRun) answer(field, text, next string) {
	r.t.Helper()
	if got := r.focus(); got != field {
		r.t.Fatalf("the focus is on %q, want %q; the screen:\n%s", got, field, r.text())
	}
	for _, ch := range text {
		_, cmd := r.m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
		r.exec(cmd)
	}
	r.key("enter")
	r.until("the question "+next+" after "+field, func() bool { return r.quit || r.focus() == next })
}

func (r *formRun) text() string {
	return ansi.Strip(r.m.render())
}

func (r *formRun) expect(wants ...string) {
	r.t.Helper()
	text := r.text()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			r.t.Fatalf("the screen lacks %q:\n%s", want, text)
		}
	}
}

// cachePlace answers the questions of a cache place up to the result of detection.
func (r *formRun) cachePlace(pathCommand string) {
	r.t.Helper()
	r.answer(keyName, "chain", keyExecutable)
	r.answer(keyExecutable, "chain", keyPlace)
	r.answer(keyPlace, "cache", keyHolds)
	r.answer(keyHolds, "", keyPathCommand)
	r.answer(keyPathCommand, pathCommand, keyPathEnv)
	r.answer(keyPathEnv, "", keyPathDefault)
	r.answer(keyPathDefault, "", keyChoice)
}

func TestTheFormSavesTheEntryOfItsAnswersAndNeverCleans(t *testing.T) {
	r := newFormRun(t)
	r.cachePlace("chain cache-dir")
	r.expect("3 · detection", "The check passed", "path  "+home+"/.cache/chain")

	r.answer(keyChoice, "", keyMeasureBy)
	r.answer(keyMeasureBy, "", keyChoice)
	r.expect("4 · space check", "size  1.5 kB")

	r.answer(keyChoice, "", keyCleanBy)
	r.expect("Nothing is run now.")
	r.answer(keyCleanBy, "", keyCleanCommand)
	r.answer(keyCleanCommand, "chain clean", keyAnother)
	r.answer(keyAnother, "", keySave)
	r.expect(`name = "chain"`, "places = 1", "Save this entry?")
	if len(r.saved) != 0 {
		t.Fatal("the entry was saved before the confirmation")
	}
	r.answer(keySave, "", "")

	want := entry.Entry{Kind: entry.Tool, Name: "chain", Executable: "chain", Places: []entry.Place{{
		Name:    "cache",
		Holds:   entry.Cache,
		Path:    &entry.Path{Command: "chain cache-dir"},
		Measure: entry.Measure{By: entry.DiskUsage},
		Clean:   entry.Clean{Command: "chain clean"},
	}}}
	if len(r.saved) != 1 || !reflect.DeepEqual(r.saved[0], want) {
		t.Fatalf("saved\n%#v\nwant\n%#v", r.saved, want)
	}
	if r.m.saved != "/registry/chain.toml" || r.m.err != nil {
		t.Fatalf("the form reports %q, %v", r.m.saved, r.m.err)
	}
	for _, argv := range r.fake.Ran() {
		if line := strings.Join(argv, " "); line != "chain cache-dir" {
			t.Fatalf("the form ran %q", line)
		}
	}
	if removed := r.fake.Removed(); len(removed) != 0 {
		t.Fatalf("the form removed %q", removed)
	}
}

func TestAFailedDetectionIsShownAndTheAnswersCanBeChanged(t *testing.T) {
	r := newFormRun(t)
	r.fake.Script([]string{"chain", "where"}, fake.Fail(3, "no configuration"))
	r.cachePlace("chain where")
	r.expect("The check failed", "`chain where`: exit status 3", "change the answers", "keep them")
	if len(r.saved) != 0 {
		t.Fatal("a failed check saved the entry")
	}

	// The first choice is to change the answers: the questions of the step come back
	// with what was typed.
	r.answer(keyChoice, "", keyPathCommand)
	r.expect("3 · detection", "chain where")
	if r.m.place.PathCommand != "chain where" {
		t.Fatalf("the answer is now %q", r.m.place.PathCommand)
	}
	if len(r.saved) != 0 {
		t.Fatal("changing the answers saved the entry")
	}
}

func TestAFailedDetectionCanBeKept(t *testing.T) {
	r := newFormRun(t)
	r.fake.Script([]string{"chain", "where"}, fake.Fail(3, "no configuration"))
	r.cachePlace("chain where")
	r.key("down")
	r.answer(keyChoice, "", keyMeasureBy)
	r.expect("4 · space check")
}

func TestAnswersThatBreakARuleStayOnTheirStep(t *testing.T) {
	r := newFormRun(t)
	r.answer(keyName, "chain", keyExecutable)
	r.answer(keyExecutable, "chain", keyPlace)
	r.answer(keyPlace, "cache", keyHolds)
	r.answer(keyHolds, "", keyPathCommand)
	r.answer(keyPathCommand, "", keyPathEnv)
	r.answer(keyPathEnv, "", keyPathDefault)
	r.answer(keyPathDefault, "", keyPathCommand)
	r.expect("3 · detection", "✗ path gives no source")
	if ran := r.fake.Ran(); len(ran) != 0 {
		t.Fatalf("commands ran: %q", ran)
	}
}

func TestTheFormRefusesANameOfTheUserAndAllowsAShippedOne(t *testing.T) {
	r := newFormRun(t)
	r.taken = []string{"mine"}
	for _, ch := range "mine" {
		r.m.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	r.key("enter")
	r.until("the refusal", func() bool { return strings.Contains(r.text(), `you already have an entry named "mine"`) })
	if got := r.focus(); got != keyName {
		t.Fatalf("the focus moved to %q", got)
	}

	r = newFormRun(t)
	r.answer(keyName, shippedName, keyExecutable)
	r.answer(keyExecutable, "", keyPlace)
	r.expect("the new entry replaces the shipped entry " + shippedName)
}

func TestLeavingTheFormWritesNothing(t *testing.T) {
	for _, key := range []string{"esc", "ctrl+c"} {
		r := newFormRun(t)
		r.cachePlace("chain cache-dir")
		r.key(key)
		r.until("the end after "+key, func() bool { return r.quit })
		if len(r.saved) != 0 || r.m.saved != "" {
			t.Fatalf("%s saved %v", key, r.saved)
		}
	}
}

func TestDecliningTheConfirmationWritesNothing(t *testing.T) {
	r := newFormRun(t)
	r.cachePlace("chain cache-dir")
	r.answer(keyChoice, "", keyMeasureBy)
	r.answer(keyMeasureBy, "", keyChoice)
	r.answer(keyChoice, "", keyCleanBy)
	r.key("down")
	r.answer(keyCleanBy, "", keyAnother)
	r.answer(keyAnother, "", keySave)
	r.key("n")
	r.until("the end", func() bool { return r.quit })
	if len(r.saved) != 0 || r.m.saved != "" {
		t.Fatalf("saved %v", r.saved)
	}
}

func TestAVersionsPlaceAsksHowVersionsAreListedAndShowsThem(t *testing.T) {
	r := newFormRun(t)
	r.answer(keyName, "chain", keyExecutable)
	r.answer(keyExecutable, "chain", keyPlace)
	r.answer(keyPlace, "versions", keyHolds)
	r.key("down")
	r.answer(keyHolds, "", keyPathCommand)
	r.answer(keyPathCommand, "chain home", keyPathEnv)
	r.answer(keyPathEnv, "", keyPathDefault)
	r.answer(keyPathDefault, "", keyListCommand)
	r.answer(keyListCommand, "chain list", keyListPattern)
	r.answer(keyListPattern, `^(\S+)`, keyVersionDir)
	r.answer(keyVersionDir, "<path>/<version>", keyChoice)
	r.expect("The check passed", "path  "+home+"/.chain", "version  v1", "version  v2")

	r.answer(keyChoice, "", keyMeasureBy)
	r.answer(keyMeasureBy, "", keyChoice)
	text := r.text()
	for _, want := range []string{"300 B", "400 B"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the screen lacks %q:\n%s", want, text)
		}
	}
}
