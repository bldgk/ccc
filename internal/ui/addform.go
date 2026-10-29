package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bldgk/ccc/internal/entry"
)

// AddForm is what the form for adding a tool works with.
type AddForm struct {
	Scanner Scanner
	// OS is the operating system a default path is asked for.
	OS string
	// UserHas reports whether the user has an entry of that name already.
	UserHas func(name string) (bool, error)
	// Shipped reports whether a shipped entry has that name.
	Shipped func(name string) bool
	// Text gives the text of the entry's file.
	Text func(entry.Entry) (string, error)
	// Save writes the entry's file and returns its path.
	Save func(entry.Entry) (string, error)
}

type addStep int

const (
	stepEntry addStep = iota
	stepPlace
	stepDetection
	stepSpace
	stepClean
	stepMore
	stepSave
)

func (s addStep) title() string {
	switch s {
	case stepEntry:
		return "1 · the entry"
	case stepPlace:
		return "2 · a place"
	case stepDetection:
		return "3 · detection"
	case stepSpace:
		return "4 · space check"
	case stepClean:
		return "5 · clean"
	}
	return "6 · save"
}

type addPhase int

const (
	// phaseAsk: the questions of the step.
	phaseAsk addPhase = iota
	// phaseChecking: the engine runs the step for the place.
	phaseChecking
	// phaseChecked: the result of the check, and what to do with the answers.
	phaseChecked
)

const (
	maxFormWidth = 76
	// The rows around the questions of a step: title, step header, notices, hint.
	formChrome = 8
	// The rows around the text of the file on the last step, besides its own panel.
	textChrome = 16
	maxShown   = 8
)

// Field keys, by which a test finds the question that has the focus.
const (
	keyName           = "name"
	keyExecutable     = "executable"
	keyPlace          = "place"
	keyHolds          = "holds"
	keyPathCommand    = "path.command"
	keyPathEnv        = "path.env"
	keyPathDefault    = "path.default"
	keyListCommand    = "list.command"
	keyListPattern    = "list.pattern"
	keyVersionDir     = "measure.path"
	keyMeasureBy      = "measure.by"
	keyMeasureCommand = "measure.command"
	keyMeasurePattern = "measure.pattern"
	keyCleanBy        = "clean.by"
	keyCleanCommand   = "clean.command"
	keyAnother        = "another"
	keySave           = "save"
	keyChoice         = "choice"
)

type checkMsg struct {
	run   int
	check Check
}

// AddModel is the form for adding a tool.
type AddModel struct {
	deps AddForm
	spin spinner.Model
	w, h int

	answers ToolAnswers
	place   PlaceAnswers
	step    addStep
	phase   addPhase
	form    *huh.Form

	// problems are the rules the answers of the current step break.
	problems []string
	// replaces is set when the name is the name of a shipped entry.
	replaces bool

	// run numbers the checks, so that the result of one that was left is dropped.
	run    int
	cancel context.CancelFunc
	check  Check

	cleanByCommand bool
	change         bool
	another        bool
	save           bool

	text    []string
	textTop int

	saved string
	err   error
}

func NewAddModel(deps AddForm) *AddModel {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(cAccent)))
	m := &AddModel{deps: deps, spin: sp, w: 80, h: 24}
	m.answers.OS = deps.OS
	return m
}

// RunAdd shows the form until the entry is saved or the user leaves. It returns the file
// it wrote; the path is empty when the user left without saving.
func RunAdd(deps AddForm) (string, error) {
	final, err := tea.NewProgram(NewAddModel(deps)).Run()
	if err != nil {
		return "", err
	}
	m := final.(*AddModel)
	return m.saved, m.err
}

func (m *AddModel) Init() tea.Cmd { return m.open() }

func newPlace() PlaceAnswers {
	return PlaceAnswers{Holds: entry.Cache, MeasureBy: entry.DiskUsage}
}

func formTheme(bool) *huh.Styles {
	t := huh.ThemeBase(true)
	button := lipgloss.NewStyle().Padding(0, 2).MarginRight(1)

	t.Focused.Base = t.Focused.Base.BorderForeground(cAccent)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = sAccent
	t.Focused.NoteTitle = sAccent
	t.Focused.Description = sDim
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(cRose)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(cRose)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(cAccent).Bold(true).SetString("▸ ")
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(cMint)
	t.Focused.FocusedButton = button.Foreground(lipgloss.Color("#282A36")).Background(cAccent).Bold(true)
	t.Focused.BlurredButton = button.Foreground(cDim)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(cMint)
	t.Focused.TextInput.Placeholder = sDim
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(cSky)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = sBold
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")

	t.Group.Title = sGroup
	t.Group.Description = sDim
	return t
}

func formKeys() *huh.KeyMap {
	keys := huh.NewDefaultKeyMap()
	// Esc leaves the form, so it cannot also belong to the filter of a list.
	keys.Select.Filter.SetEnabled(false)
	return keys
}

func (m *AddModel) formSize() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: min(max(m.w, 30), maxFormWidth), Height: max(6, m.h-formChrome)}
}

// open shows the questions of the current step and phase.
func (m *AddModel) open() tea.Cmd {
	m.form = huh.NewForm(m.groups()...).
		WithTheme(huh.ThemeFunc(formTheme)).
		WithKeyMap(formKeys()).
		WithShowHelp(false)
	init := m.form.Init()
	m.form.Update(m.formSize())
	return init
}

func input(key, title, description string, value *string) *huh.Input {
	return huh.NewInput().Key(key).Title(title).Description(description).Value(value)
}

func (m *AddModel) groups() []*huh.Group {
	if m.phase == phaseChecked {
		options := []huh.Option[bool]{huh.NewOption("continue", false), huh.NewOption("change the answers", true)}
		title := "The check passed"
		if m.check.Failure != "" {
			options = []huh.Option[bool]{huh.NewOption("change the answers", true), huh.NewOption("keep them", false)}
			title = "The check failed"
		}
		m.change = options[0].Value
		return []*huh.Group{huh.NewGroup(
			huh.NewSelect[bool]().Key(keyChoice).Title(title).Options(options...).Value(&m.change),
		)}
	}

	switch m.step {
	case stepEntry:
		return []*huh.Group{huh.NewGroup(
			input(keyName, "Name of the entry", "also the name of its file", &m.answers.Name).
				Validate(func(name string) error { return NameProblem(strings.TrimSpace(name), m.deps.UserHas) }),
			input(keyExecutable, "Executable of the tool", "looked up on PATH · empty for a tool without commands", &m.answers.Executable),
		)}
	case stepPlace:
		return []*huh.Group{huh.NewGroup(
			input(keyPlace, "Name of the place", "", &m.place.Name).
				Validate(func(name string) error { return m.answers.PlaceNameProblem(strings.TrimSpace(name)) }),
			huh.NewSelect[entry.Holds]().Key(keyHolds).Title("What it holds").
				Options(huh.NewOption("a cache", entry.Cache), huh.NewOption("installed versions", entry.Versions)).
				Value(&m.place.Holds),
		)}
	case stepDetection:
		fields := []huh.Field{
			input(keyPathCommand, "Command of the tool that prints the path", "path.command", &m.place.PathCommand),
			input(keyPathEnv, "Environment variable that holds the path", "path.env", &m.place.PathEnv),
			input(keyPathDefault, "Default path on "+m.deps.OS, "path.default."+m.deps.OS+" · may start with ~ or $NAME", &m.place.PathDefault),
		}
		if m.place.Holds == entry.Versions {
			fields = append(fields,
				input(keyListCommand, "Command that lists the versions", "list.command", &m.place.ListCommand),
				input(keyListPattern, "Pattern that takes the version name from a line", "list.pattern · a regular expression with one group", &m.place.ListPattern),
				input(keyVersionDir, "Directory of one version", "measure.path · starts with "+entry.PathPlaceholder+" and contains "+entry.VersionPlaceholder, &m.place.VersionDir),
			)
		}
		return []*huh.Group{huh.NewGroup(fields...).Description("Where the place is: any of the three sources, at least one.")}
	case stepSpace:
		return []*huh.Group{
			huh.NewGroup(
				huh.NewSelect[entry.MeasureBy]().Key(keyMeasureBy).Title("How the size is obtained").
					Options(huh.NewOption("disk usage of the path", entry.DiskUsage), huh.NewOption("a command of the tool", entry.Command)).
					Value(&m.place.MeasureBy),
			),
			huh.NewGroup(
				input(keyMeasureCommand, "Command whose output holds the size", "measure.command", &m.place.MeasureCommand),
				input(keyMeasurePattern, "Pattern that takes the size from the output", "measure.pattern · a regular expression with one group", &m.place.MeasurePattern),
			).WithHideFunc(func() bool { return m.place.MeasureBy != entry.Command }),
		}
	case stepClean:
		return []*huh.Group{
			huh.NewGroup(
				huh.NewSelect[bool]().Key(keyCleanBy).Title("How the place is cleaned").Description("Nothing is run now.").
					Options(huh.NewOption("the tool's own command", true), huh.NewOption("removal of the directory", false)).
					Value(&m.cleanByCommand),
			),
			huh.NewGroup(
				input(keyCleanCommand, "Clean command", "clean.command · never run by this form", &m.place.CleanCommand),
			).WithHideFunc(func() bool { return !m.cleanByCommand }),
		}
	case stepMore:
		m.another = false
		return []*huh.Group{huh.NewGroup(
			huh.NewConfirm().Key(keyAnother).Title("Add another place?").Affirmative("yes").Negative("no").Value(&m.another),
		)}
	}
	m.save = true
	return []*huh.Group{huh.NewGroup(
		huh.NewConfirm().Key(keySave).Title("Save this entry?").Affirmative("save").Negative("leave without saving").Value(&m.save),
	)}
}

func (m *AddModel) leave() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	return m, tea.Quit
}

func (m *AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.form != nil {
			m.form.Update(m.formSize())
		}
		return m, nil
	case checkMsg:
		if msg.run != m.run || m.phase != phaseChecking {
			return m, nil
		}
		m.check = msg.check
		m.phase = phaseChecked
		return m, m.open()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.phase != phaseChecking {
			return m, nil
		}
		return m, cmd
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m.leave()
		}
		if m.phase == phaseChecking {
			return m, nil
		}
		if m.step == stepSave && m.scrollText(msg.String()) {
			return m, nil
		}
	}

	if m.form == nil || m.phase == phaseChecking {
		return m, nil
	}
	_, cmd := m.form.Update(msg)
	if m.form.State == huh.StateCompleted {
		return m, m.advance()
	}
	return m, cmd
}

func (m *AddModel) scrollText(key string) bool {
	page := max(1, m.textRows()-1)
	switch key {
	case "up":
		m.textTop--
	case "down":
		m.textTop++
	case "pgup":
		m.textTop -= page
	case "pgdown":
		m.textTop += page
	default:
		return false
	}
	m.textTop = max(0, min(m.textTop, len(m.text)-m.textRows()))
	return true
}

func trim(values ...*string) {
	for _, v := range values {
		*v = strings.TrimSpace(*v)
	}
}

// advance acts on the answers of the form that was just completed.
func (m *AddModel) advance() tea.Cmd {
	if m.phase == phaseChecked {
		m.phase = phaseAsk
		if !m.change {
			m.step++
		}
		return m.open()
	}

	p := &m.place
	m.problems = nil
	switch m.step {
	case stepEntry:
		trim(&m.answers.Name, &m.answers.Executable)
		m.replaces = m.deps.Shipped(m.answers.Name)
		m.place = newPlace()
		m.step = stepPlace
	case stepPlace:
		trim(&p.Name)
		// A tool without an executable has no commands, so the choice of the clean step
		// starts on removal for it.
		m.cleanByCommand = m.answers.Executable != ""
		m.step = stepDetection
	case stepDetection:
		trim(&p.PathCommand, &p.PathEnv, &p.PathDefault, &p.ListCommand, &p.ListPattern, &p.VersionDir)
		if m.problems = m.answers.Problems(*p, throughDetection); len(m.problems) > 0 {
			break
		}
		return m.runCheck(CheckDetection)
	case stepSpace:
		if p.MeasureBy != entry.Command {
			p.MeasureCommand, p.MeasurePattern = "", ""
		}
		trim(&p.MeasureCommand, &p.MeasurePattern)
		if m.problems = m.answers.Problems(*p, throughSpace); len(m.problems) > 0 {
			break
		}
		return m.runCheck(CheckSpace)
	case stepClean:
		if !m.cleanByCommand {
			p.CleanCommand = ""
		}
		trim(&p.CleanCommand)
		if m.cleanByCommand && p.CleanCommand == "" {
			m.problems = []string{"the clean command is required when the place is cleaned by a command"}
			break
		}
		if m.problems = m.answers.Problems(*p, throughClean); len(m.problems) > 0 {
			break
		}
		m.step = stepMore
	case stepMore:
		m.answers.Places = append(m.answers.Places, *p)
		m.place = newPlace()
		if m.another {
			m.step = stepPlace
			break
		}
		text, err := m.deps.Text(m.answers.Entry())
		if err != nil {
			m.err = err
			return tea.Quit
		}
		m.text = strings.Split(strings.TrimRight(text, "\n"), "\n")
		m.textTop = 0
		m.step = stepSave
	case stepSave:
		if m.save {
			m.saved, m.err = m.deps.Save(m.answers.Entry())
		}
		return tea.Quit
	}
	return m.open()
}

func (m *AddModel) runCheck(check func(context.Context, Scanner, ToolAnswers, PlaceAnswers) Check) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.run++
	m.phase = phaseChecking
	m.form = nil
	run, scanner, answers, place := m.run, m.deps.Scanner, m.answers, m.place
	return tea.Batch(func() tea.Msg {
		return checkMsg{run: run, check: check(ctx, scanner, answers, place)}
	}, m.spin.Tick)
}

func (m *AddModel) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "ccc add"
	return v
}

func (m *AddModel) render() string {
	w := max(m.w, 30)
	title := m.step.title()
	rule := max(0, w-lipgloss.Width(title)-5)
	out := []string{
		titleBar(w, sDim.Render("add a tool")),
		"",
		sDim.Render("── ") + sGroup.Render(title) + " " + sDim.Render(strings.Repeat("─", rule)),
	}
	if m.replaces {
		out = append(out, sWarn.Render(ansi.Truncate("the new entry replaces the shipped entry "+m.answers.Name, w, "…")))
	}
	for _, problem := range m.problems {
		out = append(out, sErr.Render(ansi.Truncate("✗ "+oneLine(problem), w, "…")))
	}
	out = append(out, "")

	hint := "enter next · shift+tab back · esc leave"
	switch {
	case m.phase == phaseChecking:
		what := "running detection"
		if m.step == stepSpace {
			what = "measuring"
		}
		out = append(out, m.spin.View()+sDim.Render(what))
		hint = "esc leave"
	case m.phase == phaseChecked:
		out = append(out, m.checkPanel(w), "", m.form.View())
	case m.step == stepSave:
		out = append(out, m.textPanel(w), "", m.form.View())
		hint = "←→ choose · enter confirm · ↑↓ scroll the text · esc leave"
	default:
		out = append(out, m.form.View())
	}
	return strings.Join(append(out, "", sDim.Render(ansi.Truncate(hint, w, "…"))), "\n")
}

func panelBox(w int) (lipgloss.Style, int) {
	inner := max(20, min(w-8, maxPanelWidth))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(1, 2).MaxWidth(w), inner
}

// checkPanel shows what the check of the step gave: the reason of a failure, the path,
// and the versions or the sizes.
func (m *AddModel) checkPanel(w int) string {
	box, inner := panelBox(w)
	var lines []string
	if m.check.Failure != "" {
		lines = append(lines, sErr.Width(inner).Render("✗ "+oneLine(m.check.Failure)))
	}
	if m.check.Path != "" {
		lines = append(lines, sDim.Render("path  ")+ansi.Truncate(m.check.Path, inner-6, "…"))
	}
	rows := m.check.Rows
	if m.step == stepDetection {
		rows = nil
		for _, version := range m.check.Versions() {
			rows = append(rows, CheckRow{Version: version})
		}
	}
	for i, row := range rows {
		if i == maxShown {
			lines = append(lines, sDim.Render(fmt.Sprintf("… %d more", len(rows)-maxShown)))
			break
		}
		var size string
		switch {
		case m.step == stepDetection:
		case row.Measured:
			size = sBold.Render(FormatSize(row.Size))
		default:
			size = sWarn.Render("not measured")
		}
		switch {
		case row.Version == "":
			lines = append(lines, sDim.Render("size  ")+size)
		case size == "":
			lines = append(lines, sDim.Render("version  ")+sVersion.Render(ansi.Truncate(row.Version, inner-9, "…")))
		default:
			name := ansi.Truncate(row.Version, max(4, inner-lipgloss.Width(size)-1), "…")
			gap := max(1, inner-lipgloss.Width(name)-lipgloss.Width(size))
			lines = append(lines, sVersion.Render(name)+strings.Repeat(" ", gap)+size)
		}
	}
	return box.Render(strings.Join(lines, "\n"))
}

// textRows is how many lines of the file's text fit on the screen.
func (m *AddModel) textRows() int {
	return max(3, min(len(m.text), m.h-textChrome))
}

func (m *AddModel) textPanel(w int) string {
	box, inner := panelBox(w)
	rows := m.textRows()
	top := max(0, min(m.textTop, len(m.text)-rows))
	end := min(len(m.text), top+rows)
	var lines []string
	if top > 0 {
		lines = append(lines, sDim.Render(fmt.Sprintf("↑ %d more", top)))
	}
	for _, text := range m.text[top:end] {
		lines = append(lines, ansi.Truncate(text, inner, "…"))
	}
	if end < len(m.text) {
		lines = append(lines, sDim.Render(fmt.Sprintf("↓ %d more", len(m.text)-end)))
	}
	return box.Render(strings.Join(lines, "\n"))
}
