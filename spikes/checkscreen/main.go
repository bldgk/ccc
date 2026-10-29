// Command checkscreen is a spike of the `ccc check` screen. It only detects
// and measures; it contains no code that removes anything.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

func main() {
	plain := flag.Bool("plain", false, "print results as aligned text after all measurements finish")
	snapshot := flag.Bool("snapshot", false, "render the final screen once as text and exit")
	demoHang := flag.Bool("demo-hang", false, "add a fake entry whose command outlives the timeout")
	timeout := flag.Duration("timeout", 10*time.Second, "per-command and per-measurement timeout")
	flag.Parse()

	ctx := context.Background()
	store := NewStore(builtinPlaces(*demoHang))
	events := Start(ctx, store.Places, *timeout)

	if *plain || *snapshot || !term.IsTerminal(os.Stdout.Fd()) {
		for e := range events {
			store.Apply(e)
		}
		if *snapshot {
			m := newModel(store, nil)
			m.done = true
			m.cursor = 2
			for _, r := range m.rows() {
				if r.Kind == KindCache && r.State == Sized {
					m.marked[r.mk] = true
				}
			}
			fmt.Println(ansi.Strip(m.render()))
			return
		}
		writePlain(os.Stdout, store)
		return
	}

	if _, err := tea.NewProgram(newModel(store, events)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
