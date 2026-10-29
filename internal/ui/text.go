package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/bldgk/ccc/internal/engine"
)

// Output wraps a destination of text so that colour reaches it only when it is a
// terminal that shows colour and the environment does not turn colour off.
func Output(w io.Writer, environ []string) io.Writer {
	return colorprofile.NewWriter(w, environ)
}

// WriteProblems prints one line per problem.
func WriteProblems(w io.Writer, problems []Problem) {
	for _, p := range problems {
		fmt.Fprintf(w, "%s: %s\n", p.Source, oneLine(p.Reason))
	}
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text)))
}

// WriteRows prints the rows as aligned text: entry, place, version, and the size or the
// state with its reason.
func WriteRows(w io.Writer, rows []engine.Row) {
	var entryW, placeW, versionW int
	for _, row := range rows {
		entryW = max(entryW, ansi.StringWidth(row.Entry))
		placeW = max(placeW, ansi.StringWidth(placeText(row)))
		versionW = max(versionW, ansi.StringWidth(row.Version))
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%s  %s  %s  %s\n",
			pad(row.Entry, entryW), pad(placeText(row), placeW), pad(row.Version, versionW), stateText(row))
	}
}

// CleanEverything scans, cleans every row that "clean everything" covers and prints one
// line per row as it finishes, then the totals.
func CleanEverything(ctx context.Context, eng Engine, w io.Writer) Totals {
	rows := eng.Everything(Collect(ctx, eng))
	var entryW, labelW int
	for _, row := range rows {
		entryW = max(entryW, ansi.StringWidth(row.Entry))
		labelW = max(labelW, ansi.StringWidth(label(row)))
	}
	var totals Totals
	for result := range eng.Clean(ctx, rows) {
		totals.add(result)
		fmt.Fprintf(w, "%s  %s  %s\n",
			sGroup.Render(pad(result.Row.Entry, entryW)), pad(label(result.Row), labelW), resultText(result))
	}
	fmt.Fprintln(w, totalsText(totals))
	return totals
}

func resultText(result engine.Result) string {
	switch {
	case result.Outcome == engine.Failed:
		return sErr.Render("failed: " + oneLine(result.Message))
	case result.FreedKnown:
		return "freed " + sBold.Render(FormatSize(result.Freed))
	default:
		return sWarn.Render("cleaned, freed size unknown: " + oneLine(result.Message))
	}
}

func totalsText(totals Totals) string {
	text := "total freed " + sBold.Render(FormatSize(totals.Freed)) + ", " + fmt.Sprintf("%d failed", totals.Failed)
	if totals.Unknown > 0 {
		text += fmt.Sprintf(", %d with freed size unknown", totals.Unknown)
	}
	return text
}
