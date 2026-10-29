// Package ui is the terminal side of ccc: the interactive screen and the text outputs.
// It shows what the engine reports and decides nothing about detection, sizes or
// cleaning.
package ui

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
)

// Engine is what the outputs need from the engine, already bound to the entries it
// works on.
type Engine interface {
	// Places lists the places a scan is going to look at.
	Places() []engine.Place
	// Scan delivers one result per place as each finishes and closes the channel at
	// the end.
	Scan(ctx context.Context) <-chan engine.PlaceResult
	// Clean cleans the rows in order and delivers one result per row.
	Clean(ctx context.Context, rows []engine.Row) <-chan engine.Result
	// Everything selects the rows that "clean everything" covers.
	Everything(rows []engine.Row) []engine.Row
}

// Problem is a source of entries that was left out, and why.
type Problem struct {
	Source string
	Reason string
}

type rowKey struct {
	entry, place, version int
}

func keyOf(row engine.Row) rowKey {
	return rowKey{row.EntryIndex, row.PlaceIndex, row.VersionIndex}
}

func compareRows(a, b engine.Row) int {
	return cmp.Or(
		cmp.Compare(a.EntryIndex, b.EntryIndex),
		cmp.Compare(a.PlaceIndex, b.PlaceIndex),
		cmp.Compare(a.VersionIndex, b.VersionIndex),
	)
}

// Collect runs a scan to its end and returns the rows in their stable order.
func Collect(ctx context.Context, eng Engine) []engine.Row {
	var rows []engine.Row
	for result := range eng.Scan(ctx) {
		rows = append(rows, result.Rows...)
	}
	slices.SortFunc(rows, compareRows)
	return rows
}

// placeText names the place of a row: the path for a folder, the name of the place
// otherwise.
func placeText(row engine.Row) string {
	if row.Kind == entry.Folder {
		return row.Path
	}
	return row.Place
}

func label(row engine.Row) string {
	if row.Version != "" {
		return placeText(row) + " " + row.Version
	}
	return placeText(row)
}

func holdsVersions(row engine.Row) bool {
	return row.Holds == entry.Versions
}

// stateText is the size of a measured row, or its state with the reason.
func stateText(row engine.Row) string {
	if row.State == engine.Measured {
		return FormatSize(row.Size)
	}
	if row.Reason == "" {
		return row.State.String()
	}
	return row.State.String() + " (" + oneLine(row.Reason) + ")"
}

// oneLine keeps a message of a tool, which may span lines, on the line of its row.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// Totals sums up the results of cleaning.
type Totals struct {
	// Freed is the sum over the rows whose freed size is known.
	Freed int64
	// Failed counts the rows that were not cleaned.
	Failed int
	// Unknown counts the rows that were cleaned without a known freed size; they are
	// not part of Freed.
	Unknown int
}

func (t *Totals) add(result engine.Result) {
	switch {
	case result.Outcome == engine.Failed:
		t.Failed++
	case result.FreedKnown:
		t.Freed += result.Freed
	default:
		t.Unknown++
	}
}
