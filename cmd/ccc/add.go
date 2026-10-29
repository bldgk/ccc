package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bldgk/ccc/internal/engine"
	"github.com/bldgk/ccc/internal/entry"
	"github.com/bldgk/ccc/internal/registry"
	"github.com/bldgk/ccc/internal/sys/host"
	"github.com/bldgk/ccc/internal/ui"
)

const nameFlag = "--name"

// add reads the arguments of `ccc add`: no argument opens the form for a tool, a path
// adds a folder.
func (a app) add(args []string) int {
	var path, name string
	var hasPath, hasName bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value, inline := strings.CutPrefix(arg, nameFlag+"=")
		switch {
		case arg == nameFlag && i+1 < len(args) && !hasName:
			i++
			name, hasName = args[i], true
		case inline && !hasName:
			name, hasName = value, true
		case !strings.HasPrefix(arg, "-") && arg != "" && !hasPath:
			path, hasPath = arg, true
		default:
			fmt.Fprint(a.stderr, usage)
			return exitUsage
		}
	}
	switch {
	case hasPath:
		return a.report(a.addFolder(path, name, hasName))
	case hasName:
		fmt.Fprint(a.stderr, usage)
		return exitUsage
	}
	return a.report(a.addTool())
}

func (a app) report(err error) int {
	if err != nil {
		fmt.Fprintln(a.stderr, "ccc add:", err)
		return exitFailed
	}
	return exitOK
}

// userDir is the user's registry directory on the machine the program works with.
func (a app) userDir() (string, error) {
	xdg, _ := a.system.Getenv("XDG_CONFIG_HOME")
	return registry.UserDir(xdg, a.system.HomeDir)
}

// userHas reports whether a file of an entry of that name is in the user's registry
// directory. Whatever is at that path counts, because writing never replaces it.
func (a app) userHas(dir, name string) (bool, error) {
	file, err := registry.FilePath(dir, name)
	if err != nil {
		return false, err
	}
	_, err = a.system.Stat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (a app) addFolder(path, name string, named bool) error {
	home, err := a.system.HomeDir()
	if err != nil {
		return fmt.Errorf("home directory: %w", err)
	}
	if !filepath.IsAbs(home) {
		return fmt.Errorf("home directory %q is not an absolute path", home)
	}
	home = filepath.Clean(home)

	if !filepath.IsAbs(path) {
		dir, err := a.workDir()
		if err != nil {
			return fmt.Errorf("working directory: %w", err)
		}
		path = filepath.Join(dir, path)
	}
	path = filepath.Clean(path)

	if err := a.removable(path, home); err != nil {
		return fmt.Errorf("refusing to add %s: %w", path, err)
	}
	// In the file of an entry $NAME is an environment variable, so a path with a $
	// would be read back as a different directory.
	if strings.Contains(path, "$") {
		return fmt.Errorf("refusing to add %s: an entry cannot hold a path that contains $", path)
	}
	if !named {
		name = filepath.Base(path)
	}
	stored := path
	if rest, inside := strings.CutPrefix(path, home+string(filepath.Separator)); inside {
		stored = "~" + string(filepath.Separator) + rest
	}
	e := entry.Entry{Kind: entry.Folder, Name: name, Path: stored}

	dir, err := a.userDir()
	if err != nil {
		return err
	}
	taken, err := a.userHas(dir, name)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("you already have an entry named %q; give another name with %s", name, nameFlag)
	}
	shipped, err := registry.Shipped()
	if err != nil {
		return err
	}
	replaces := slices.ContainsFunc(shipped, func(s entry.Entry) bool { return s.Name == name })
	// A name taken from the path replaces a shipped entry by accident; only a name the
	// user typed does it on purpose.
	if replaces && !named {
		return fmt.Errorf("%q is the name of a shipped entry, which this folder would replace; give another name with %s", name, nameFlag)
	}
	eng, err := engine.New(a.system, a.settings)
	if err != nil {
		return err
	}
	file, err := registry.Write(a.system, dir, e)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "wrote", file)
	if replaces {
		fmt.Fprintf(a.stdout, "the new entry replaces the shipped entry %s\n", name)
	}

	var rows []engine.Row
	for result := range eng.Scan(context.Background(), []entry.Entry{e}) {
		rows = append(rows, result.Rows...)
	}
	if len(rows) == 0 {
		fmt.Fprintf(a.stdout, "%s  %s  the directory does not exist yet\n", name, path)
		return nil
	}
	ui.WriteRows(ui.Output(a.stdout, a.environ), rows)
	return nil
}

// removable applies to a folder, before it becomes an entry, the removal guards that can
// be checked without the directory being there: where the path lies, and what is at it
// when something is.
func (a app) removable(path, home string) error {
	if err := host.CheckLocation(path, home); err != nil {
		return err
	}
	info, err := a.system.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Symlink {
		return errors.New("the path is a symbolic link")
	}
	if !info.Dir {
		return errors.New("the path is not a directory")
	}
	return nil
}

func (a app) addTool() error {
	dir, err := a.userDir()
	if err != nil {
		return err
	}
	if !a.inputTerminal || !a.terminal {
		return fmt.Errorf("the form needs a terminal; an entry can also be written by hand as a TOML file in %s", dir)
	}
	shipped, err := registry.Shipped()
	if err != nil {
		return err
	}
	eng, err := engine.New(a.system, a.settings)
	if err != nil {
		return err
	}
	file, err := a.form(ui.AddForm{
		Scanner: eng,
		OS:      a.system.OS(),
		UserHas: func(name string) (bool, error) { return a.userHas(dir, name) },
		Shipped: func(name string) bool {
			for _, e := range shipped {
				if e.Name == name {
					return true
				}
			}
			return false
		},
		Text: func(e entry.Entry) (string, error) {
			data, err := registry.Encode(e)
			return string(data), err
		},
		Save: func(e entry.Entry) (string, error) { return registry.Write(a.system, dir, e) },
	})
	if err != nil {
		return err
	}
	if file != "" {
		fmt.Fprintln(a.stdout, "wrote", file)
	}
	return nil
}
