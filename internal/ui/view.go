package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bldgk/ccc/internal/engine"
)

const (
	sizeWidth     = 11
	maxBarWidth   = 22
	maxPanelWidth = 60
)

type line struct {
	text string
	// row is the index of the row the line shows, -1 for a line that is not a row.
	row int
}

func titleBar(w int, status string) string {
	word := "ccc"
	cols := lipgloss.Blend1D(len(word), cAccent, cSky)
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

func (m *Model) bodyLines(w int) []line {
	if len(m.rows) == 0 {
		if m.scanning {
			return nil
		}
		return []line{{"", -1}, {sDim.Render("nothing found"), -1}}
	}

	var largest int64 = 1
	labelW := 4
	for _, row := range m.rows {
		if !row.pending && !holdsVersions(row.Row) && row.State == engine.Measured && row.Size > largest {
			largest = row.Size
		}
		labelW = max(labelW, lipgloss.Width(label(row.Row)))
	}
	fixed := 2 + 2 + 2 + sizeWidth
	labelW = min(labelW, max(8, w-fixed-2))
	barW := min(maxBarWidth, w-fixed-labelW-2)
	if barW < 6 {
		barW = 0
	}
	grad := lipgloss.Blend1D(max(barW, 2), cMint, cSun, cRose)

	var out []line
	for i, item := range m.rows {
		row := item.Row
		if i == 0 || row.EntryIndex != m.rows[i-1].EntryIndex {
			rule := max(0, w-lipgloss.Width(row.Entry)-5)
			out = append(out, line{"", -1},
				line{sDim.Render("── ") + sGroup.Render(row.Entry) + " " + sDim.Render(strings.Repeat("─", rule)), -1})
		}
		cur := "  "
		if i == m.cursor {
			cur = sAccent.Render("▸ ")
		}
		versions := holdsVersions(row)
		marked := m.marked[keyOf(row)]
		var mark string
		switch {
		case item.pending || !canMark(row):
			mark = sDim.Render("· ")
		case marked && versions:
			mark = sVersion.Render("◆ ")
		case marked:
			mark = sOK.Render("● ")
		case versions:
			mark = sVersion.Render("◇ ")
		default:
			mark = sDim.Render("○ ")
		}
		text := ansi.Truncate(label(row), labelW, "…")
		style := lipgloss.NewStyle()
		if versions {
			style = sVersion
		}
		if i == m.cursor {
			style = style.Bold(true)
		}
		labelCell := style.Render(text) + strings.Repeat(" ", labelW-lipgloss.Width(text))

		var right string
		switch {
		case item.pending:
			right = m.spin.View() + sDim.Render("measuring")
		case row.State == engine.Measured:
			size := FormatSize(row.Size)
			right = strings.Repeat(" ", max(0, sizeWidth-lipgloss.Width(size))) + sBold.Render(size)
			if versions {
				right += "  " + sVersion.Render("version")
			} else if barW > 0 {
				right += "  " + bar(row.Size, largest, barW, grad)
			}
		case row.State == engine.NotMeasured:
			right = sWarn.Render("… " + stateText(row))
		case row.State == engine.Undetermined:
			right = sErr.Render("✗ " + stateText(row))
		}
		out = append(out, line{ansi.Truncate(cur+mark+labelCell+"  "+right, w, "…"), i})
	}
	return out
}

// bar draws a size against the largest one on a square-root scale, so that a small row
// next to a very large one still shows.
func bar(size, largest int64, width int, grad []color.Color) string {
	n := int(float64(width)*math.Sqrt(float64(size)/float64(largest)) + 0.5)
	if size > 0 && n == 0 {
		n = 1
	}
	var b strings.Builder
	for k := range width {
		if k < n {
			b.WriteString(lipgloss.NewStyle().Foreground(grad[k]).Render("█"))
		} else {
			b.WriteString(sDim.Render("·"))
		}
	}
	return b.String()
}

// sizeSum adds up the rows that have a size and counts the ones that have none.
func sizeSum(rows []engine.Row) (sum int64, unmeasured int) {
	for _, row := range rows {
		if row.State == engine.Measured {
			sum += row.Size
		} else {
			unmeasured++
		}
	}
	return sum, unmeasured
}

func (m *Model) status() string {
	if pending := m.pending(); pending > 0 {
		return m.spin.View() + sDim.Render(fmt.Sprintf("measuring %d", pending))
	}
	return sOK.Render("✓ done")
}

func (m *Model) footer(w int) []string {
	marked := m.markedRows()
	sum, unmeasured := sizeSum(marked)
	left := sBold.Render(fmt.Sprintf("marked %d", len(marked))) + sDim.Render(" · ") + sAccent.Render(FormatSize(sum))
	if unmeasured > 0 {
		left += sDim.Render(" · ") + sWarn.Render(fmt.Sprintf("%d not measured", unmeasured))
	}
	var covered []engine.Row
	for _, row := range m.eng.Everything(m.known()) {
		if row.State == engine.Measured {
			covered = append(covered, row)
		}
	}
	all, _ := sizeSum(covered)
	more := ""
	if m.pending() > 0 {
		more = "+"
	}
	right := sDim.Render("all caches ") + sBold.Render(FormatSize(all)+more)
	gap := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	hints := "↑↓/jk move · g/G first/last · space mark · a all caches · enter clean · q quit"
	if lipgloss.Width(hints) > w {
		hints = "jk move · g/G · space mark · a all · ⏎ clean · q"
	}
	return []string{
		sDim.Render(strings.Repeat("─", w)),
		ansi.Truncate(left+strings.Repeat(" ", gap)+right, w, ""),
		sDim.Render(ansi.Truncate(hints, w, "…")),
	}
}

func (m *Model) render() string {
	w := max(m.w, 30)
	head := titleBar(w, m.status())
	if m.stage != stageList {
		return head + "\n" + m.panel(w)
	}

	var under []string
	for _, p := range m.problems {
		under = append(under, sErr.Render(ansi.Truncate("✗ "+p.Source+": "+oneLine(p.Reason), w, "…")))
	}
	foot := m.footer(w)

	lines := m.bodyLines(w)
	avail := len(lines)
	if m.h > 0 {
		avail = max(3, m.h-2-len(foot)-len(under))
	}
	// A list that does not fit gives up two lines to say how many rows are out of view.
	clipped := len(lines) > avail
	win := avail
	if clipped {
		win = avail - 2
	}
	cursorLine := 0
	for i, l := range lines {
		if l.row == m.cursor {
			cursorLine = i
		}
	}
	if cursorLine < m.top {
		m.top = cursorLine
		// The header of the group comes into view together with its first row.
		for m.top > 0 && lines[m.top-1].row == -1 {
			m.top--
		}
	}
	if cursorLine >= m.top+win {
		m.top = cursorLine - win + 1
	}
	m.top = max(0, min(m.top, max(0, len(lines)-win)))
	// Rows are not shown at the top without the header of their group, unless the cursor is among them.
	for clipped && m.top > 0 && m.top < cursorLine && lines[m.top].row >= 0 {
		m.top++
	}
	end := min(len(lines), m.top+win)
	// A group header is not left at the bottom without any of its rows.
	for clipped && end > m.top+1 && lines[end-1].row == -1 {
		end--
	}
	view := lines[m.top:end]
	above, below := 0, 0
	for _, l := range lines[:m.top] {
		if l.row >= 0 {
			above++
		}
	}
	for _, l := range lines[end:] {
		if l.row >= 0 {
			below++
		}
	}

	var b strings.Builder
	b.WriteString(head + "\n")
	if clipped {
		if above > 0 {
			b.WriteString(sDim.Render(fmt.Sprintf("↑ %d more", above)))
		}
		b.WriteString("\n")
	}
	for _, l := range view {
		b.WriteString(l.text + "\n")
	}
	for i := len(view); m.h > 0 && i < win; i++ {
		b.WriteString("\n")
	}
	if clipped {
		if below > 0 {
			b.WriteString(sDim.Render(fmt.Sprintf("↓ %d more", below)))
		}
		b.WriteString("\n")
	}
	for _, l := range under {
		b.WriteString(l + "\n")
	}
	b.WriteString(strings.Join(foot, "\n"))
	return b.String()
}

type panelLine struct {
	text string
	// item is the index of the marked row the line belongs to.
	item int
}

func (m *Model) itemState(i int) (state string, message string) {
	item := m.items[i]
	switch {
	case m.stage == stageConfirm:
		if item.row.State == engine.Measured {
			return sBold.Render(FormatSize(item.row.Size)), ""
		}
		return sWarn.Render(item.row.State.String()), ""
	case item.done && item.result.Outcome == engine.Failed:
		return sErr.Render("✗ failed"), sErr.Render(oneLine(item.result.Message))
	case item.done && item.result.FreedKnown:
		return sOK.Render("✓ freed ") + sBold.Render(FormatSize(item.result.Freed)), ""
	case item.done:
		return sOK.Render("✓ ") + sWarn.Render("freed size unknown"), sWarn.Render(oneLine(item.result.Message))
	case m.stage == stageCleaning && i == m.current:
		return m.spin.View() + sDim.Render("cleaning"), ""
	case m.stage == stageCleaning:
		return sDim.Render("waiting"), ""
	}
	return sDim.Render("not cleaned"), ""
}

func (m *Model) panel(w int) string {
	inner := max(20, min(w-8, maxPanelWidth))

	var lines []panelLine
	for i, item := range m.items {
		state, message := m.itemState(i)
		nameW := max(4, inner-lipgloss.Width(state)-1)
		name := ansi.Truncate(item.row.Entry+" · "+label(item.row), nameW, "…")
		gap := max(1, inner-lipgloss.Width(name)-lipgloss.Width(state))
		lines = append(lines, panelLine{name + strings.Repeat(" ", gap) + state, i})
		if message != "" {
			lines = append(lines, panelLine{"  " + ansi.Truncate(message, inner-2, "…"), i})
		}
	}

	var closing []string
	var hint string
	switch m.stage {
	case stageConfirm:
		rows := make([]engine.Row, len(m.items))
		for i, item := range m.items {
			rows[i] = item.row
		}
		sum, unmeasured := sizeSum(rows)
		total := sAccent.Render("total  " + FormatSize(sum))
		if unmeasured > 0 {
			total += sDim.Render(" · ") + sWarn.Render(fmt.Sprintf("%d not measured", unmeasured))
		}
		closing = []string{sDim.Render(strings.Repeat("─", inner)), total}
		hint = "y clean · any other key back"
	case stageCleaning:
		hint = "ctrl+c stop after the current row"
		if m.stopping {
			hint = "stopping after the current row"
		}
	case stageResult:
		totals := m.totals()
		failed := sDim
		if totals.Failed > 0 {
			failed = sErr
		}
		total := sAccent.Render("freed  "+FormatSize(totals.Freed)) + sDim.Render(" · ") + failed.Render(fmt.Sprintf("%d failed", totals.Failed))
		if totals.Unknown > 0 {
			total += sDim.Render(" · ") + sWarn.Render(fmt.Sprintf("%d with freed size unknown", totals.Unknown))
		}
		if left := len(m.items) - m.current; left > 0 {
			total += sDim.Render(" · " + fmt.Sprintf("%d not cleaned", left))
		}
		closing = []string{sDim.Render(strings.Repeat("─", inner)), ansi.Truncate(total, inner, "…")}
		hint = "press any key to scan again"
	}

	// Lines around the panel's rows: title, blank, border and padding, the closing
	// lines, blank, hint, and the two lines that say how many rows are out of view.
	budget := len(lines)
	if m.h > 0 {
		budget = max(1, m.h-(1+1+4+len(closing)+2+2))
	}
	start, end := 0, len(lines)
	if len(lines) > budget {
		focus := 0
		for i, l := range lines {
			if l.item == min(m.current, len(m.items)-1) && m.stage != stageConfirm {
				focus = i
				break
			}
		}
		start = max(0, min(focus-budget/2, len(lines)-budget))
		end = start + budget
	}
	above, below := 0, 0
	if start > 0 {
		above = lines[start].item
		if lines[start-1].item == lines[start].item {
			above++
		}
	}
	if end < len(lines) {
		below = len(m.items) - 1 - lines[end-1].item
	}

	var content []string
	if above > 0 {
		content = append(content, sDim.Render(fmt.Sprintf("↑ %d more", above)))
	}
	for _, l := range lines[start:end] {
		content = append(content, l.text)
	}
	if below > 0 {
		content = append(content, sDim.Render(fmt.Sprintf("↓ %d more", below)))
	}
	content = append(content, closing...)

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(1, 2).MaxWidth(w)
	return "\n" + box.Render(strings.Join(content, "\n")) + "\n\n" + sDim.Render(hint)
}
