package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/bldgk/ccc/internal/entry"
)

type Outcome int

const (
	Cleaned Outcome = iota
	Failed
)

func (o Outcome) String() string {
	if o == Cleaned {
		return "cleaned"
	}
	return "failed"
}

// Result is what cleaning one row came to.
type Result struct {
	Row     Row
	Outcome Outcome
	// Freed is the size before minus the size after. It is meaningful only when
	// FreedKnown is set: a row that was cleaned but whose size before or after could
	// not be measured has no number, and Message says which of the two is missing.
	Freed      int64
	FreedKnown bool
	// Message is the reason of a failure, or why the freed size is not known.
	Message string
}

// Clean cleans the rows one after another, in the order given, and delivers the result
// of each. A row that fails does not stop the next. The channel is closed after the last
// row, or before the next row once ctx has ended.
func (e *Engine) Clean(ctx context.Context, rows []Row) <-chan Result {
	out := make(chan Result)
	go func() {
		defer close(out)
		for _, row := range rows {
			if ctx.Err() != nil {
				return
			}
			result := e.cleanRow(ctx, row)
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (e *Engine) cleanRow(ctx context.Context, row Row) Result {
	t := row.target
	failed := func(err error) Result {
		return Result{Row: row, Outcome: Failed, Message: err.Error()}
	}

	if row.State == Undetermined {
		return failed(fmt.Errorf("the place could not be determined: %s", row.Reason))
	}

	// What a row stands for depends on whether the tool was installed when it was
	// scanned, so a row is cleaned only while that is still true.
	onPath := e.onPath(t)
	if row.installed && !onPath {
		return failed(fmt.Errorf("%s is no longer on PATH: the tool disappeared since the scan, scan again", t.executable))
	}
	if !row.installed && onPath {
		return failed(fmt.Errorf("%s is on PATH now and was not at the scan: scan again", t.executable))
	}

	if !onPath {
		if row.Dir == "" {
			return failed(errors.New("the executable is not on PATH and the row has no directory to remove"))
		}
		if err := e.removeDir(ctx, row, onPath); err != nil {
			return failed(err)
		}
		return e.freed(ctx, row)
	}

	if t.place.Holds == entry.Versions && row.Version == "" {
		return failed(errors.New("the row names no version"))
	}
	if err := e.requires(ctx, t); err != nil {
		return failed(err)
	}
	if before := t.place.Clean.Before; before != "" {
		argv, err := command(before, row.placePath, row.Version)
		if err != nil {
			return failed(err)
		}
		if _, err := e.run(ctx, argv, e.settings.CleanTimeout); err != nil {
			return failed(fmt.Errorf("clean.before: %w", err))
		}
	}

	switch {
	case t.place.Clean.Command != "":
		argv, err := command(t.place.Clean.Command, row.placePath, row.Version)
		if err != nil {
			return failed(err)
		}
		if _, err := e.run(ctx, argv, e.settings.CleanTimeout); err != nil {
			return failed(err)
		}
	case row.Dir != "":
		if err := e.removeDir(ctx, row, onPath); err != nil {
			return failed(err)
		}
	default:
		return failed(errors.New("the place has no clean.command and the row has no directory to remove"))
	}

	return e.freed(ctx, row)
}

// removeDir removes the directory of a row. Removal cannot be undone: the place is
// resolved again first, and the directory is removed only while detection still gives
// this row the directory the scan showed.
func (e *Engine) removeDir(ctx context.Context, row Row, onPath bool) error {
	resolved, reasons := e.resolvePath(ctx, row.target, onPath)
	if resolved == "" {
		return fmt.Errorf("the place no longer resolves to a path: %s", strings.Join(reasons, "; "))
	}
	now := resolved
	if row.installed {
		dir, err := rowDir(row.target, resolved, row.Version)
		if err != nil {
			return err
		}
		now = dir
	}
	if now != row.Dir {
		return fmt.Errorf("the row was scanned at %q and now resolves to %q", row.Dir, now)
	}
	return e.sys.RemoveDir(row.Dir)
}

func (e *Engine) freed(ctx context.Context, row Row) Result {
	result := Result{Row: row, Outcome: Cleaned}
	if row.State != Measured {
		result.Message = "the size before cleaning is not known: " + row.Reason
		return result
	}

	if row.Dir != "" {
		_, err := e.sys.Stat(row.Dir)
		if errors.Is(err, fs.ErrNotExist) {
			result.Freed = row.Size
			result.FreedKnown = true
			return result
		}
	}
	var (
		after int64
		err   error
	)
	if row.installed {
		after, err = e.measure(ctx, row.target, row.placePath, row.Dir, row.Version)
	} else {
		after, err = e.diskUsage(ctx, row.Dir)
	}
	if err != nil {
		result.Message = "the size after cleaning is not known: " + err.Error()
		return result
	}
	result.Freed = row.Size - after
	result.FreedKnown = true
	return result
}
