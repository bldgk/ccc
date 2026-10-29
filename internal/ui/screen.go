package ui

import (
	"context"
	"slices"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bldgk/ccc/internal/engine"
)

type stage int

const (
	// stageList: the rows, marking.
	stageList stage = iota
	// stageConfirm: the panel with the marked rows, waiting for the answer.
	stageConfirm
	// stageCleaning: the panel while rows are cleaned.
	stageCleaning
	// stageResult: the panel after the last row.
	stageResult
)

// cleanItem is one marked row on its way through cleaning.
type cleanItem struct {
	row    engine.Row
	done   bool
	result engine.Result
}

// listRow is one line of the list: a row of the scan, or a place that is still being
// measured.
type listRow struct {
	engine.Row
	pending bool
}

type placeMsg struct {
	scan   int
	result engine.PlaceResult
}

type scanDoneMsg struct {
	scan int
}

type resultMsg struct {
	index  int
	result engine.Result
}

// Model is the interactive screen.
type Model struct {
	eng      Engine
	problems []Problem
	spin     spinner.Model

	rows   []listRow
	marked map[rowKey]bool
	cursor int
	top    int
	w, h   int

	// scan numbers the scans, so that a row of a scan that was replaced is dropped.
	scan       int
	scanning   bool
	results    <-chan engine.PlaceResult
	cancelScan context.CancelFunc

	stage    stage
	items    []cleanItem
	current  int
	stopping bool
}

func NewModel(eng Engine, problems []Problem) *Model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(cAccent)))
	return &Model{eng: eng, problems: problems, spin: sp, marked: map[rowKey]bool{}, w: 80}
}

// Run shows the screen until the user quits.
func Run(eng Engine, problems []Problem) error {
	_, err := tea.NewProgram(NewModel(eng, problems)).Run()
	return err
}

func (m *Model) Init() tea.Cmd { return m.startScan() }

func (m *Model) startScan() tea.Cmd {
	if m.cancelScan != nil {
		m.cancelScan()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelScan = cancel
	m.scan++
	m.scanning = true
	m.rows = nil
	for _, place := range m.eng.Places() {
		m.rows = append(m.rows, listRow{pending: true, Row: engine.Row{
			Entry:      place.Entry,
			Place:      place.Name,
			Kind:       place.Kind,
			Path:       place.Path,
			EntryIndex: place.EntryIndex,
			PlaceIndex: place.PlaceIndex,
		}})
	}
	slices.SortFunc(m.rows, compareListRows)
	m.marked = map[rowKey]bool{}
	m.cursor, m.top = 0, 0
	m.stage = stageList
	m.items = nil
	m.results = m.eng.Scan(ctx)
	return tea.Batch(m.waitPlace(), m.spin.Tick)
}

func (m *Model) waitPlace() tea.Cmd {
	scan, results := m.scan, m.results
	return func() tea.Msg {
		if result, ok := <-results; ok {
			return placeMsg{scan: scan, result: result}
		}
		return scanDoneMsg{scan: scan}
	}
}

func compareListRows(a, b listRow) int {
	return compareRows(a.Row, b.Row)
}

// place puts the rows of a place where its pending row was. The cursor stays on the row
// it was on, or on the first row of the place when it was on the pending one.
func (m *Model) place(result engine.PlaceResult) {
	for i, row := range m.rows {
		if row.pending && row.EntryIndex == result.Place.EntryIndex && row.PlaceIndex == result.Place.PlaceIndex {
			m.rows = slices.Delete(m.rows, i, i+1)
			if m.cursor > i {
				m.cursor--
			}
			break
		}
	}
	for _, row := range result.Rows {
		arrived := listRow{Row: row}
		at, _ := slices.BinarySearchFunc(m.rows, arrived, compareListRows)
		if at < m.cursor {
			m.cursor++
		}
		m.rows = slices.Insert(m.rows, at, arrived)
	}
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))
}

// known returns the rows the scan has delivered so far.
func (m *Model) known() []engine.Row {
	var rows []engine.Row
	for _, row := range m.rows {
		if !row.pending {
			rows = append(rows, row.Row)
		}
	}
	return rows
}

func (m *Model) pending() int {
	return len(m.rows) - len(m.known())
}

func canMark(row engine.Row) bool {
	return row.State != engine.Undetermined
}

func (m *Model) markedRows() []engine.Row {
	var rows []engine.Row
	for _, row := range m.known() {
		if m.marked[keyOf(row)] {
			rows = append(rows, row)
		}
	}
	return rows
}

// toggleEverything marks every row that "clean everything" covers and can be marked, or
// unmarks them when all of them are marked already.
func (m *Model) toggleEverything() {
	var covered []engine.Row
	all := true
	for _, row := range m.eng.Everything(m.known()) {
		if !canMark(row) {
			continue
		}
		covered = append(covered, row)
		if !m.marked[keyOf(row)] {
			all = false
		}
	}
	for _, row := range covered {
		if all {
			delete(m.marked, keyOf(row))
		} else {
			m.marked[keyOf(row)] = true
		}
	}
}

// cleanRow cleans one row with a context that never ends: stopping takes effect
// between rows, so the row in progress finishes.
func (m *Model) cleanRow(index int) tea.Cmd {
	row := m.items[index].row
	return func() tea.Msg {
		result, ok := <-m.eng.Clean(context.Background(), []engine.Row{row})
		if !ok {
			result = engine.Result{Row: row, Outcome: engine.Failed, Message: "cleaning ended without a result for this row"}
		}
		return resultMsg{index: index, result: result}
	}
}

func (m *Model) spinning() bool {
	return m.scanning || m.stage == stageCleaning
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case placeMsg:
		if msg.scan != m.scan {
			return m, nil
		}
		m.place(msg.result)
		return m, m.waitPlace()
	case scanDoneMsg:
		if msg.scan == m.scan {
			m.scanning = false
		}
	case resultMsg:
		if m.stage != stageCleaning || msg.index != m.current {
			return m, nil
		}
		m.items[msg.index].done = true
		m.items[msg.index].result = msg.result
		m.current++
		if m.current == len(m.items) || m.stopping {
			m.stage = stageResult
			return m, nil
		}
		return m, m.cleanRow(m.current)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if !m.spinning() {
			return m, nil
		}
		return m, cmd
	case tea.KeyPressMsg:
		return m.key(msg.String())
	}
	return m, nil
}

func (m *Model) quit() (tea.Model, tea.Cmd) {
	if m.cancelScan != nil {
		m.cancelScan()
	}
	return m, tea.Quit
}

func (m *Model) key(key string) (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageConfirm:
		if key != "y" {
			m.stage = stageList
			m.items = nil
			return m, nil
		}
		m.stage = stageCleaning
		m.current = 0
		m.stopping = false
		return m, tea.Batch(m.cleanRow(0), m.spin.Tick)
	case stageCleaning:
		if key == "ctrl+c" {
			m.stopping = true
		}
		return m, nil
	case stageResult:
		if key == "ctrl+c" {
			return m.quit()
		}
		return m, m.startScan()
	}

	switch key {
	case "q", "ctrl+c":
		return m.quit()
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = max(0, min(len(m.rows)-1, m.cursor+1))
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.rows)-1)
	case "space":
		if m.cursor < len(m.rows) && !m.rows[m.cursor].pending && canMark(m.rows[m.cursor].Row) {
			k := keyOf(m.rows[m.cursor].Row)
			if m.marked[k] {
				delete(m.marked, k)
			} else {
				m.marked[k] = true
			}
		}
	case "a":
		m.toggleEverything()
	case "enter":
		rows := m.markedRows()
		if len(rows) == 0 {
			return m, nil
		}
		m.items = make([]cleanItem, len(rows))
		for i, row := range rows {
			m.items[i].row = row
		}
		m.stage = stageConfirm
	}
	return m, nil
}

func (m *Model) totals() Totals {
	var totals Totals
	for _, item := range m.items {
		if item.done {
			totals.add(item.result)
		}
	}
	return totals
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "ccc"
	return v
}
