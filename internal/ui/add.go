package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
)

// Scanner is the part of the engine the form checks its answers with.
type Scanner interface {
	Scan(ctx context.Context, entries []entry.Entry) <-chan engine.PlaceResult
}

// PlaceAnswers is what the form asks about one place.
type PlaceAnswers struct {
	Name  string
	Holds entry.Holds

	PathCommand string
	PathEnv     string
	PathDefault string
	ListCommand string
	ListPattern string
	// VersionDir is the directory of one installed version, written with <path> and
	// <version>.
	VersionDir string

	MeasureBy      entry.MeasureBy
	MeasureCommand string
	MeasurePattern string

	// CleanCommand is empty when the place is cleaned by removing its directory.
	CleanCommand string
}

// ToolAnswers is everything the form has been told.
type ToolAnswers struct {
	Name       string
	Executable string
	Places     []PlaceAnswers
	// OS is the operating system the default path is given for.
	OS string
}

// answered says how far a place has been asked about.
type answered int

const (
	// throughDetection: where the place is and, for versions, how they are listed.
	throughDetection answered = iota
	// throughSpace: also how its size is obtained.
	throughSpace
	// throughClean: everything.
	throughClean
)

// place turns the answers into a place of an entry. What has not been asked yet takes the
// simplest legal form: the size by disk usage, cleaning by removal.
func (p PlaceAnswers) place(os string, upTo answered) entry.Place {
	place := entry.Place{Name: p.Name, Holds: p.Holds}
	path := &entry.Path{Command: p.PathCommand, Env: p.PathEnv}
	if p.PathDefault != "" {
		path.Default = map[string]string{os: p.PathDefault}
	}
	place.Path = path
	place.Measure = entry.Measure{By: entry.DiskUsage}
	if p.Holds == entry.Versions {
		place.List = &entry.List{Command: p.ListCommand, Pattern: p.ListPattern}
		place.Measure.Path = p.VersionDir
	}
	if upTo >= throughSpace && p.MeasureBy == entry.Command {
		place.Measure.By = entry.Command
		place.Measure.Command = p.MeasureCommand
		place.Measure.Pattern = p.MeasurePattern
	}
	if upTo >= throughClean {
		place.Clean.Command = p.CleanCommand
	}
	return place
}

// Entry is the entry the answers describe.
func (a ToolAnswers) Entry() entry.Entry {
	e := entry.Entry{Kind: entry.Tool, Name: a.Name, Executable: a.Executable}
	for _, p := range a.Places {
		e.Places = append(e.Places, p.place(a.OS, throughClean))
	}
	return e
}

// probe is the entry of one place alone, as far as it has been answered.
func (a ToolAnswers) probe(p PlaceAnswers, upTo answered) entry.Entry {
	return entry.Entry{
		Kind:       entry.Tool,
		Name:       a.Name,
		Executable: a.Executable,
		Places:     []entry.Place{p.place(a.OS, upTo)},
	}
}

// Problems lists the rules of an entry that the place breaks as far as it has been
// answered; an empty list means the answers can be checked and saved.
func (a ToolAnswers) Problems(p PlaceAnswers, upTo answered) []string {
	err := entry.Validate(a.probe(p, upTo))
	if err == nil {
		return nil
	}
	prefix := fmt.Sprintf("place %q: ", p.Name)
	var problems []string
	for line := range strings.SplitSeq(err.Error(), "\n") {
		problems = append(problems, strings.TrimPrefix(line, prefix))
	}
	return problems
}

// NameProblem says why an entry cannot take a name; nil means it can.
func NameProblem(name string, userHas func(string) (bool, error)) error {
	if name == "" {
		return errors.New("the name is required")
	}
	taken, err := userHas(name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("you already have an entry named %q", name)
	}
	return nil
}

// PlaceNameProblem says why a place cannot take a name inside the entry; nil means it can.
func (a ToolAnswers) PlaceNameProblem(name string) error {
	if name == "" {
		return errors.New("the name is required")
	}
	if slices.ContainsFunc(a.Places, func(p PlaceAnswers) bool { return p.Name == name }) {
		return fmt.Errorf("another place of this entry is named %q", name)
	}
	return nil
}

// CheckRow is one row the engine gave for the place being checked.
type CheckRow struct {
	// Version is empty for a row that is not an installed version.
	Version  string
	Measured bool
	Size     int64
	// Reason says why the row has no size.
	Reason string
}

// Check is what running a step for one place gave.
type Check struct {
	// Path is the path detection resolved; empty when it resolved none.
	Path string
	Rows []CheckRow
	// Failure is the reason the step failed; empty when it passed.
	Failure string
}

// Versions are the names of the installed versions the check listed.
func (c Check) Versions() []string {
	var versions []string
	for _, row := range c.Rows {
		if row.Version != "" {
			versions = append(versions, row.Version)
		}
	}
	return versions
}

// CheckDetection runs detection for one place through the engine and reports the path it
// resolved and the versions it listed.
func CheckDetection(ctx context.Context, scanner Scanner, a ToolAnswers, p PlaceAnswers) Check {
	check, _ := scanPlace(ctx, scanner, a, p, throughDetection)
	return check
}

// CheckSpace runs detection and the space check for one place through the engine and
// reports the size of every row.
func CheckSpace(ctx context.Context, scanner Scanner, a ToolAnswers, p PlaceAnswers) Check {
	check, rows := scanPlace(ctx, scanner, a, p, throughSpace)
	if check.Failure != "" {
		return check
	}
	for _, row := range rows {
		if row.State != engine.Measured {
			check.Failure = strings.TrimSpace(row.Version + " " + stateText(row))
			break
		}
	}
	return check
}

// scanPlace scans the one-place entry and reads off what both checks share: the path,
// the rows, and the failures that mean detection itself did not work.
func scanPlace(ctx context.Context, scanner Scanner, a ToolAnswers, p PlaceAnswers, upTo answered) (Check, []engine.Row) {
	var rows []engine.Row
	for result := range scanner.Scan(ctx, []entry.Entry{a.probe(p, upTo)}) {
		rows = append(rows, result.Rows...)
	}
	slices.SortFunc(rows, compareRows)

	var check Check
	if len(rows) == 0 {
		check.Failure = "the place was not found on this machine: no directory exists at the path its sources give"
		if p.Holds == entry.Versions {
			check.Failure += ", or no version is listed"
		}
		return check, nil
	}
	check.Path = rows[0].PlacePath()
	for _, row := range rows {
		check.Rows = append(check.Rows, CheckRow{
			Version:  row.Version,
			Measured: row.State == engine.Measured,
			Size:     row.Size,
			Reason:   row.Reason,
		})
	}
	switch {
	case rows[0].State == engine.Undetermined:
		check.Failure = oneLine(rows[0].Reason)
	case a.Executable != "" && !rows[0].Installed():
		check.Failure = fmt.Sprintf("%s is not on PATH, so none of the entry's commands was run", a.Executable)
	}
	return check, rows
}
