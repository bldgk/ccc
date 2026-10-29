package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"

	"github.com/bldgk/ccc/internal/entry"
)

// Place is one place a scan is going to look at.
type Place struct {
	Entry string
	// Name is empty for a folder.
	Name string
	Kind entry.Kind
	// Path is the path of a folder as its entry gives it; empty for a place of a tool,
	// whose path is known only once the scan has resolved it.
	Path       string
	EntryIndex int
	PlaceIndex int
}

// PlaceResult is what the scan found at one place.
type PlaceResult struct {
	Place Place
	// Rows is empty when the place is absent.
	Rows []Row
}

// Places lists the places a scan of the entries looks at, in the order of the rows: every
// place of every tool, and one place per folder.
func Places(entries []entry.Entry) []Place {
	targets := targetsOf(entries)
	places := make([]Place, len(targets))
	for i, t := range targets {
		places[i] = t.planned()
	}
	return places
}

func (t target) planned() Place {
	return Place{Entry: t.entryName, Name: t.place.Name, Kind: t.kind, Path: t.folderPath, EntryIndex: t.entryIndex, PlaceIndex: t.placeIndex}
}

// Scan runs detection and the space check for every place of every entry, several at a
// time, and delivers one result per place of Places as soon as that place is finished.
// The channel is closed when the scan is over or ctx ends.
func (e *Engine) Scan(ctx context.Context, entries []entry.Entry) <-chan PlaceResult {
	out := make(chan PlaceResult)
	go func() {
		defer close(out)
		slots := make(chan struct{}, e.settings.Parallel)
		var wg sync.WaitGroup
		defer wg.Wait()
		for _, t := range targetsOf(entries) {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-slots }()
				result := PlaceResult{Place: t.planned(), Rows: e.scanPlace(ctx, t)}
				select {
				case out <- result:
				case <-ctx.Done():
				}
			}()
		}
	}()
	return out
}

// scanPlace returns the rows of one place: none when the place is absent.
func (e *Engine) scanPlace(ctx context.Context, t target) []Row {
	var rows []Row
	emit := func(row Row) {
		row.Path = row.Dir
		if row.Path == "" {
			row.Path = row.placePath
		}
		rows = append(rows, row)
	}
	e.detect(ctx, t, emit)
	return rows
}

func (e *Engine) detect(ctx context.Context, t target, emit func(Row)) {
	onPath := e.onPath(t)

	placePath := ""
	if t.isDirectory() {
		resolved, reasons := e.resolvePath(ctx, t, onPath)
		if resolved == "" {
			if onPath {
				row := t.row("", "", 0, onPath)
				row.State = Undetermined
				row.Reason = strings.Join(reasons, "; ")
				emit(row)
			}
			return
		}
		info, err := e.sys.Stat(resolved)
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		if err != nil {
			row := t.row(resolved, "", 0, onPath)
			row.State = NotMeasured
			row.Reason = err.Error()
			emit(row)
			return
		}
		if !info.Dir && !info.Symlink {
			return
		}
		placePath = resolved
	} else if !onPath {
		return
	}

	// The tool is not installed and its directory is still there: none of the entry's
	// commands is run, and the place is one row for the whole directory.
	if !onPath {
		row := t.row(placePath, "", 0, false)
		row.Dir = placePath
		size, err := e.diskUsage(ctx, placePath)
		if err != nil {
			row.State = NotMeasured
			row.Reason = err.Error()
		} else {
			row.State = Measured
			row.Size = size
		}
		emit(row)
		return
	}

	versions := []string{""}
	if t.place.Holds == entry.Versions {
		listed, err := e.listVersions(ctx, t)
		if err != nil {
			row := t.row(placePath, "", 0, onPath)
			row.State = Undetermined
			row.Reason = err.Error()
			emit(row)
			return
		}
		versions = listed
	}
	if len(versions) == 0 {
		return
	}

	requiresErr := e.requires(ctx, t)
	for i, version := range versions {
		row := t.row(placePath, version, i, onPath)
		dir, err := rowDir(t, placePath, version)
		switch {
		case err != nil:
			row.State = NotMeasured
			row.Reason = err.Error()
		case requiresErr != nil:
			row.Dir = dir
			row.State = NotMeasured
			row.Reason = requiresErr.Error()
		default:
			row.Dir = dir
			size, err := e.measure(ctx, t, placePath, dir, version)
			if err != nil {
				row.State = NotMeasured
				row.Reason = err.Error()
			} else {
				row.State = Measured
				row.Size = size
			}
		}
		emit(row)
	}
}

func (e *Engine) requires(ctx context.Context, t target) error {
	if t.place.Requires == nil {
		return nil
	}
	argv, err := command(t.place.Requires.Command, "", "")
	if err != nil {
		return err
	}
	if _, err := e.run(ctx, argv, e.settings.CommandTimeout); err != nil {
		return fmt.Errorf("requires: %w", err)
	}
	return nil
}

func (e *Engine) listVersions(ctx context.Context, t target) ([]string, error) {
	pattern, err := regexp.Compile(t.place.List.Pattern)
	if err != nil {
		return nil, fmt.Errorf("list.pattern: %w", err)
	}
	if pattern.NumSubexp() != 1 {
		return nil, fmt.Errorf("list.pattern has %d groups, not one", pattern.NumSubexp())
	}
	argv, err := command(t.place.List.Command, "", "")
	if err != nil {
		return nil, err
	}
	out, err := e.run(ctx, argv, e.settings.CommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	var versions []string
	seen := map[string]bool{}
	for line := range strings.Lines(out) {
		m := pattern.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil || m[1] == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		versions = append(versions, m[1])
	}
	return versions, nil
}

// measure obtains the size of one row.
func (e *Engine) measure(ctx context.Context, t target, placePath, dir, version string) (int64, error) {
	switch t.place.Measure.By {
	case entry.DiskUsage:
		return e.diskUsage(ctx, dir)
	case entry.Command:
		return e.measureByCommand(ctx, t, placePath, version)
	}
	return 0, fmt.Errorf("measure.by %q is not known", t.place.Measure.By)
}

func (e *Engine) diskUsage(ctx context.Context, dir string) (int64, error) {
	if dir == "" {
		return 0, errors.New("the place has no directory to measure")
	}
	walkCtx, cancel := context.WithTimeout(ctx, e.settings.WalkTimeout)
	defer cancel()
	size, err := e.sys.DiskUsage(walkCtx, dir)
	if err != nil {
		if errors.Is(walkCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return 0, fmt.Errorf("measuring %s timed out after %s", dir, e.settings.WalkTimeout)
		}
		return 0, err
	}
	if size < 0 {
		return 0, fmt.Errorf("measuring %s gave a negative size %d", dir, size)
	}
	return size, nil
}

func (e *Engine) measureByCommand(ctx context.Context, t target, placePath, version string) (int64, error) {
	pattern, err := regexp.Compile(t.place.Measure.Pattern)
	if err != nil {
		return 0, fmt.Errorf("measure.pattern: %w", err)
	}
	if pattern.NumSubexp() != 1 {
		return 0, fmt.Errorf("measure.pattern has %d groups, not one", pattern.NumSubexp())
	}
	argv, err := command(t.place.Measure.Command, placePath, version)
	if err != nil {
		return 0, err
	}
	out, err := e.run(ctx, argv, e.settings.CommandTimeout)
	if err != nil {
		return 0, err
	}
	for line := range strings.Lines(out) {
		m := pattern.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m == nil {
			continue
		}
		size, err := ParseSize(m[1])
		if err != nil {
			return 0, fmt.Errorf("`%s`: %w", strings.Join(argv, " "), err)
		}
		return size, nil
	}
	return 0, fmt.Errorf("`%s`: no line of the output matches measure.pattern", strings.Join(argv, " "))
}
