package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// keys are the bindings of every screen. The same bindings decide what a key does and
// what the help line says, so the two cannot drift apart.
var keys = struct {
	Up, Down, First, Last, Mark, All, Clean, Help, Quit key.Binding
	// Move and Ends stand for Up and Down, and for First and Last, in the short help line.
	Move, Ends key.Binding

	Yes, Stop, ForceQuit key.Binding
	// Back and Rescan answer any key that has no binding of its own on their screen.
	Back, Rescan key.Binding
}{
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	First: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
	Last:  key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
	Mark:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "mark")),
	All:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all caches")),
	Clean: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "clean")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "more")),
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Move:  key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑↓/jk", "move")),
	Ends:  key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "first/last")),

	Yes:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "clean")),
	Stop:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "stop after the current row")),
	ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Back:      shownOnly("any other key", "back"),
	Rescan:    shownOnly("any key", "scan again"),
}

// shownOnly is a binding for the help line alone. Its key text is no key name, so it
// never matches a key press.
func shownOnly(text, action string) key.Binding {
	return key.NewBinding(key.WithKeys(text), key.WithHelp(text, action))
}

func newHelp() help.Model {
	h := help.New()
	h.Styles = help.Styles{
		Ellipsis:       sDim,
		ShortKey:       sAccent,
		ShortDesc:      sDim,
		ShortSeparator: sDim,
		FullKey:        sAccent,
		FullDesc:       sDim,
		FullSeparator:  sDim,
	}
	return h
}

// helpLines is the help line of the screen, or the full help of the list when it is open.
func (m *Model) helpLines(w int) string {
	m.help.SetWidth(w)
	switch m.stage {
	case stageConfirm:
		return m.help.ShortHelpView([]key.Binding{keys.Yes, keys.Back})
	case stageCleaning:
		if m.stopping {
			return ""
		}
		return m.help.ShortHelpView([]key.Binding{keys.Stop})
	case stageResult:
		return m.help.ShortHelpView([]key.Binding{keys.Rescan, keys.ForceQuit})
	}
	if m.fullHelp {
		less := keys.Help
		less.SetHelp("?", "less")
		return m.help.FullHelpView([][]key.Binding{
			{keys.Up, keys.Down, keys.First, keys.Last},
			{keys.Mark, keys.All},
			{keys.Clean, less, keys.Quit},
		})
	}
	return m.help.ShortHelpView([]key.Binding{keys.Move, keys.Ends, keys.Mark, keys.All, keys.Clean, keys.Help, keys.Quit})
}
