package main

import (
	"math"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	cAccent = lipgloss.Color("#FF79C6")
	cMint   = lipgloss.Color("#50FA7B")
	cSun    = lipgloss.Color("#F1FA8C")
	cRose   = lipgloss.Color("#FF5555")
	cViolet = lipgloss.Color("#BD93F9")
	cSky    = lipgloss.Color("#8BE9FD")
	cDim    = lipgloss.Color("#7F849C")

	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sBold    = lipgloss.NewStyle().Bold(true)
	sAccent  = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	sGroup   = lipgloss.NewStyle().Foreground(cSky).Bold(true)
	sVersion = lipgloss.NewStyle().Foreground(cViolet)
	sWarn    = lipgloss.NewStyle().Foreground(cSun)
	sErr     = lipgloss.NewStyle().Foreground(cRose)
	sOK      = lipgloss.NewStyle().Foreground(cMint).Bold(true)
)

type eventMsg Event
type doneMsg struct{}

type model struct {
	store  *Store
	events <-chan Event
	done   bool
	spin   spinner.Model
	marked map[string]bool // key: place index + row key
	cursor int
	top    int
	w, h   int
	review bool
}

func newModel(s *Store, ch <-chan Event) *model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(cAccent)))
	return &model{store: s, events: ch, spin: sp, marked: map[string]bool{}, w: 80}
}

func (m *model) wait() tea.Cmd {
	return func() tea.Msg {
		if e, ok := <-m.events; ok {
			return eventMsg(e)
		}
		return doneMsg{}
	}
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.wait(), m.spin.Tick) }

type line struct {
	text string
	row  int // index into rows, -1 for non-row lines
}

type flatRow struct {
	Row
	mk string
}

func (m *model) rows() []flatRow {
	var out []flatRow
	for i, rs := range m.store.Rows {
		for _, r := range rs {
			out = append(out, flatRow{r, string(rune('A'+i)) + "/" + r.Key})
		}
	}
	return out
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case eventMsg:
		m.store.Apply(Event(msg))
		return m, m.wait()
	case doneMsg:
		m.done = true
	case spinner.TickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		if m.done {
			return m, nil
		}
		return m, c
	case tea.KeyPressMsg:
		if m.review {
			m.review = false
			return m, nil
		}
		rows := m.rows()
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(rows)-1, m.cursor+1)
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(rows) - 1
		case "space":
			if r := rows[m.cursor]; r.State == Sized {
				m.marked[r.mk] = !m.marked[r.mk]
			}
		case "a":
			all := true
			for _, r := range rows {
				if r.Kind == KindCache && r.State == Sized && !m.marked[r.mk] {
					all = false
				}
			}
			for _, r := range rows {
				if r.Kind == KindCache && r.State == Sized {
					m.marked[r.mk] = !all
				}
			}
		case "enter":
			m.review = true
		}
	}
	return m, nil
}

func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "ccc check"
	return v
}

func titleBar(w int, status string) string {
	word := "ccc"
	cols := lipgloss.Blend1D(len(word), lipgloss.Color("#FF79C6"), lipgloss.Color("#8BE9FD"))
	var b strings.Builder
	for i, ch := range word {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(cols[i]).Render(string(ch)))
	}
	left := sAccent.Render("✦ ") + b.String() + sAccent.Render(" ✦") + sDim.Render("  cute cache cleaner")
	gap := w - lipgloss.Width(left) - lipgloss.Width(status)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + status
}

func (m *model) bodyLines(w int) []line {
	rows := m.rows()
	var maxCache int64 = 1
	labelW := 4
	for _, r := range rows {
		if r.Kind == KindCache && r.State == Sized && r.Bytes > maxCache {
			maxCache = r.Bytes
		}
		labelW = max(labelW, lipgloss.Width(r.Label))
	}
	const sizeW = 11
	fixed := 2 + 2 + 2 + sizeW // cursor, mark, gaps, size
	labelW = min(labelW, max(8, w-fixed-2))
	barW := min(22, w-fixed-labelW-2)
	if barW < 6 {
		barW = 0
	}
	grad := lipgloss.Blend1D(max(barW, 2), cMint, cSun, cRose)

	var out []line
	lastGroup := ""
	for i, r := range rows {
		if r.Group != lastGroup {
			lastGroup = r.Group
			head := sGroup.Render(r.Group)
			rule := max(0, w-lipgloss.Width(r.Group)-5)
			out = append(out, line{"", -1}, line{sDim.Render("── ") + head + " " + sDim.Render(strings.Repeat("─", rule)), -1})
		}
		cur := "  "
		if i == m.cursor {
			cur = sAccent.Render("▸ ")
		}
		isVer := r.Kind == KindVersion
		var mark string
		switch {
		case r.State != Sized:
			mark = sDim.Render("· ")
		case m.marked[r.mk] && isVer:
			mark = sVersion.Render("◆ ")
		case m.marked[r.mk]:
			mark = sOK.Render("● ")
		case isVer:
			mark = sVersion.Render("◇ ")
		default:
			mark = sDim.Render("○ ")
		}
		lbl := ansi.Truncate(r.Label, labelW, "…")
		lstyle := lipgloss.NewStyle()
		switch {
		case isVer:
			lstyle = sVersion
		case r.State == Absent:
			lstyle = sDim
		}
		if i == m.cursor {
			lstyle = lstyle.Bold(true)
		}
		lblCell := lstyle.Render(lbl) + strings.Repeat(" ", labelW-lipgloss.Width(lbl))

		var right string
		switch r.State {
		case Pending:
			right = m.spin.View() + sDim.Render("measuring")
		case Sized:
			sz := r.SizeText()
			cell := strings.Repeat(" ", max(0, sizeW-lipgloss.Width(sz))) + sBold.Render(sz)
			right = cell
			if isVer {
				right += "  " + sVersion.Render("version")
			} else if barW > 0 {
				n := int(float64(barW)*math.Sqrt(float64(r.Bytes)/float64(maxCache)) + 0.5)
				if r.Bytes > 0 && n == 0 {
					n = 1
				}
				var b strings.Builder
				for k := 0; k < barW; k++ {
					if k < n {
						b.WriteString(lipgloss.NewStyle().Foreground(grad[k]).Render("█"))
					} else {
						b.WriteString(sDim.Render("·"))
					}
				}
				right += "  " + b.String()
			}
		case Absent:
			right = sDim.Render("not present")
		case Undetermined:
			right = sErr.Render("✗ " + r.StatusText())
		case NotMeasured:
			right = sWarn.Render("… " + r.StatusText())
		}
		text := cur + mark + lblCell + "  " + right
		out = append(out, line{ansi.Truncate(text, w, "…"), i})
	}
	return out
}

func (m *model) totals() (marked, markedN, caches int64) {
	for _, r := range m.rows() {
		if r.State != Sized {
			continue
		}
		if r.Kind == KindCache {
			caches += r.Bytes
		}
		if m.marked[r.mk] {
			marked += r.Bytes
			markedN++
		}
	}
	return
}

func (m *model) render() string {
	w := max(m.w, 30)
	pending := m.store.Pending()
	status := sOK.Render("✓ done")
	if pending > 0 {
		status = m.spin.View() + sDim.Render("measuring "+itoa(pending))
	}
	head := titleBar(w, status)

	mk, mn, caches := m.totals()
	more := ""
	if pending > 0 {
		more = "+"
	}
	left := sBold.Render("marked "+itoa(int(mn))) + sDim.Render(" · ") + sAccent.Render(humanSize(mk))
	right := sDim.Render("all caches ") + sBold.Render(humanSize(caches)+more)
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	totals := left + strings.Repeat(" ", gap) + right
	hints := "↑↓/jk move · space mark · a all caches · enter review · q quit"
	if lipgloss.Width(hints) > w {
		hints = "jk move · space mark · a all · ⏎ · q"
	}
	foot := []string{sDim.Render(strings.Repeat("─", w)), ansi.Truncate(totals, w, ""), sDim.Render(ansi.Truncate(hints, w, "…"))}

	if m.review {
		return head + "\n" + m.reviewPanel(w) + "\n\n" + sDim.Render("press any key to go back")
	}

	lines := m.bodyLines(w)
	avail := len(lines)
	if m.h > 0 {
		avail = max(3, m.h-2-len(foot))
	}
	cl := 0
	for i, l := range lines {
		if l.row == m.cursor {
			cl = i
		}
	}
	if cl < m.top {
		m.top = cl
		if m.top > 0 && lines[m.top-1].row == -1 {
			m.top--
		}
	}
	if cl >= m.top+avail {
		m.top = cl - avail + 1
	}
	m.top = max(0, min(m.top, max(0, len(lines)-avail)))
	view := lines[m.top:min(len(lines), m.top+avail)]
	var b strings.Builder
	b.WriteString(head + "\n")
	for _, l := range view {
		b.WriteString(l.text + "\n")
	}
	for i := len(view); m.h > 0 && i < avail; i++ {
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(foot, "\n"))
	return b.String()
}

func (m *model) reviewPanel(w int) string {
	var sb strings.Builder
	var total int64
	n := 0
	for _, r := range m.rows() {
		if m.marked[r.mk] && r.State == Sized {
			name := r.Group + " · " + r.Label
			sz := r.SizeText()
			pad := max(1, min(w-8, 44)-lipgloss.Width(name)-lipgloss.Width(sz))
			sb.WriteString(name + strings.Repeat(" ", pad) + sBold.Render(sz) + "\n")
			total += r.Bytes
			n++
		}
	}
	if n == 0 {
		sb.WriteString(sDim.Render("nothing marked") + "\n")
	} else {
		sb.WriteString(sDim.Render(strings.Repeat("─", min(w-8, 44))) + "\n")
		sb.WriteString(sAccent.Render("total  "+humanSize(total)) + "\n")
	}
	sb.WriteString("\n" + sWarn.Render("This prototype does not remove anything."))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(1, 2).MaxWidth(w)
	return "\n" + box.Render(strings.TrimRight(sb.String(), "\n"))
}

func itoa(n int) string { return strconv.Itoa(n) }
