// Package engine turns entries into rows, measures them and cleans them. It reaches the
// machine only through the sys contract.
package engine

import (
	"errors"
	"time"

	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/sys"
)

type Settings struct {
	// CommandTimeout bounds every command of detection and measuring.
	CommandTimeout time.Duration
	// WalkTimeout bounds one disk-usage walk.
	WalkTimeout time.Duration
	// CleanTimeout bounds clean.before and clean.command, which delete data and
	// are slower than a query.
	CleanTimeout time.Duration
	// Parallel is how many places are detected and measured at once.
	Parallel int
}

func DefaultSettings() Settings {
	return Settings{
		CommandTimeout: 30 * time.Second,
		WalkTimeout:    5 * time.Minute,
		CleanTimeout:   10 * time.Minute,
		Parallel:       4,
	}
}

func (s Settings) validate() error {
	var errs []error
	if s.CommandTimeout <= 0 {
		errs = append(errs, errors.New("CommandTimeout must be positive"))
	}
	if s.WalkTimeout <= 0 {
		errs = append(errs, errors.New("WalkTimeout must be positive"))
	}
	if s.CleanTimeout <= 0 {
		errs = append(errs, errors.New("CleanTimeout must be positive"))
	}
	if s.Parallel <= 0 {
		errs = append(errs, errors.New("Parallel must be positive"))
	}
	return errors.Join(errs...)
}

type Engine struct {
	sys      sys.System
	settings Settings
}

func New(system sys.System, settings Settings) (*Engine, error) {
	if err := settings.validate(); err != nil {
		return nil, err
	}
	return &Engine{sys: system, settings: settings}, nil
}

type State int

const (
	// Measured: Size holds the bytes the row takes.
	Measured State = iota
	// NotMeasured: the place is there, its size is not known; Reason says why.
	NotMeasured
	// Undetermined: where the place is could not be found out; Reason says why.
	Undetermined
)

func (s State) String() string {
	switch s {
	case Measured:
		return "measured"
	case NotMeasured:
		return "not measured"
	case Undetermined:
		return "could not be determined"
	}
	return "unknown state"
}

// Row is one line of the result of a scan: a cache place, one installed version, or a
// folder.
type Row struct {
	Entry string
	// Place is empty for a folder.
	Place string
	// Version is set only for a row of a place that holds versions.
	Version string
	Kind    entry.Kind
	Holds   entry.Holds
	// EntryIndex, PlaceIndex and VersionIndex give rows a stable order, whatever
	// order they arrive in.
	EntryIndex   int
	PlaceIndex   int
	VersionIndex int
	// Dir is the directory the row stands for; empty when the row is not a directory.
	Dir string
	// Path is the path the row stands for whenever one is known: Dir, or the resolved
	// directory of its place when the row itself has no directory to measure or clean.
	// Cleaning never reads it.
	Path   string
	State  State
	Size   int64
	Reason string

	target target
	// placePath is the resolved path of the place the row belongs to.
	placePath string
	// installed records whether the scan found the executable on PATH. It decides
	// what the row is: with the tool installed the entry's commands produced it, without
	// the tool it is the whole directory of the place, measured by disk usage.
	installed bool
}

// Installed reports whether the scan found the executable of the row's entry on PATH.
// When it did not, none of the entry's commands was run for the row.
func (r Row) Installed() bool { return r.installed }

// PlacePath is the resolved path of the place the row belongs to, which for a row of one
// installed version is the directory above Dir. It is empty when no path was resolved.
func (r Row) PlacePath() string { return r.placePath }

// target is one place of one entry, in the form the three steps work on. A folder is a
// place with a fixed path, measured by disk usage and cleaned by removal.
type target struct {
	entryName  string
	kind       entry.Kind
	executable string
	place      entry.Place
	folderPath string
	entryIndex int
	placeIndex int
}

func (t target) isDirectory() bool {
	return t.kind == entry.Folder || t.place.Path != nil
}

func targetsOf(entries []entry.Entry) []target {
	var targets []target
	for i, e := range entries {
		if e.Kind == entry.Folder {
			targets = append(targets, target{
				entryName:  e.Name,
				kind:       entry.Folder,
				folderPath: e.Path,
				entryIndex: i,
				place: entry.Place{
					Holds:   entry.Cache,
					Measure: entry.Measure{By: entry.DiskUsage},
				},
			})
			continue
		}
		for j, place := range e.Places {
			targets = append(targets, target{
				entryName:  e.Name,
				kind:       e.Kind,
				executable: e.Executable,
				place:      place,
				entryIndex: i,
				placeIndex: j,
			})
		}
	}
	return targets
}

func (t target) row(placePath, version string, versionIndex int, installed bool) Row {
	return Row{
		Entry:        t.entryName,
		Place:        t.place.Name,
		Version:      version,
		Kind:         t.kind,
		Holds:        t.place.Holds,
		EntryIndex:   t.entryIndex,
		PlaceIndex:   t.placeIndex,
		VersionIndex: versionIndex,
		target:       t,
		placePath:    placePath,
		installed:    installed,
	}
}

// Everything selects what "clean everything" cleans: every row of a cache place and
// every folder row. A row of installed versions is cleaned only when it is chosen by name.
func Everything(rows []Row) []Row {
	var selected []Row
	for _, row := range rows {
		if row.Holds == entry.Cache {
			selected = append(selected, row)
		}
	}
	return selected
}
